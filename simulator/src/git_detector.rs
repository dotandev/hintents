// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{Duration, Instant};

/// Configuration for the `.git` directory search.
///
/// Controls the maximum wall-clock time spent walking up the directory tree.
/// Use [`SearchConfig::default`] for a sensible 5-second limit.
#[derive(Debug, Clone)]
pub struct SearchConfig {
    /// Maximum time to spend searching for a `.git` directory.
    pub timeout: Duration,
}

impl Default for SearchConfig {
    fn default() -> Self {
        Self {
            timeout: Duration::from_secs(5),
        }
    }
}

#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct GitRepository {
    pub remote_url: String,
    pub branch: String,
    pub commit_hash: String,
    pub root_path: PathBuf,
}

impl GitRepository {
    /// Detect the git repository containing `start_path`, using default search
    /// configuration (5-second timeout).
    pub fn detect(start_path: &Path) -> Option<Self> {
        Self::detect_with_config(start_path, &SearchConfig::default())
    }

    /// Detect the git repository containing `start_path` with a custom
    /// [`SearchConfig`].
    pub fn detect_with_config(start_path: &Path, cfg: &SearchConfig) -> Option<Self> {
        let root_path = Self::find_git_root(start_path, cfg)?;
        let remote_url = Self::get_remote_url(&root_path)?;
        let branch = Self::get_current_branch(&root_path).unwrap_or_else(|| "main".to_string());
        let commit_hash = Self::get_commit_hash(&root_path)?;

        Some(GitRepository {
            remote_url,
            branch,
            commit_hash,
            root_path,
        })
    }

    /// Walk up the directory tree from `start_path` looking for a `.git` entry.
    ///
    /// The search respects the timeout in `cfg`; if the deadline is exceeded
    /// before a `.git` directory is found, `None` is returned.  Symlinked
    /// `.git` entries are accepted.  Any directory that cannot be read due to
    /// permission errors or other I/O failures is skipped silently.
    fn find_git_root(start_path: &Path, cfg: &SearchConfig) -> Option<PathBuf> {
        let deadline = Instant::now() + cfg.timeout;
        let mut current = start_path.to_path_buf();

        loop {
            // Abort the search if we have exceeded the allowed time.
            if Instant::now() >= deadline {
                return None;
            }

            let git_dir = current.join(".git");

            // `symlink_metadata` does not follow symlinks, so we can inspect
            // both real directories and symbolic links without dereferencing
            // them.  An `Err` here normally means a permission problem or a
            // non-existent path — either way we continue climbing.
            match std::fs::symlink_metadata(&git_dir) {
                Ok(meta) => {
                    if meta.is_dir() || meta.file_type().is_symlink() {
                        return Some(current);
                    }
                }
                Err(_) => {
                    // Inaccessible or absent — keep walking up.
                }
            }

            if !current.pop() {
                return None;
            }
        }
    }

    fn get_remote_url(repo_path: &Path) -> Option<String> {
        let output = Command::new("git")
            .arg("-C")
            .arg(repo_path)
            .arg("config")
            .arg("--get")
            .arg("remote.origin.url")
            .output()
            .ok()?;

        if output.status.success() {
            let url = String::from_utf8_lossy(&output.stdout).trim().to_string();
            Some(Self::normalize_git_url(&url))
        } else {
            None
        }
    }

    fn get_current_branch(repo_path: &Path) -> Option<String> {
        let output = Command::new("git")
            .arg("-C")
            .arg(repo_path)
            .arg("rev-parse")
            .arg("--abbrev-ref")
            .arg("HEAD")
            .output()
            .ok()?;

        if output.status.success() {
            Some(String::from_utf8_lossy(&output.stdout).trim().to_string())
        } else {
            None
        }
    }

    fn get_commit_hash(repo_path: &Path) -> Option<String> {
        let output = Command::new("git")
            .arg("-C")
            .arg(repo_path)
            .arg("rev-parse")
            .arg("HEAD")
            .output()
            .ok()?;

        if output.status.success() {
            Some(String::from_utf8_lossy(&output.stdout).trim().to_string())
        } else {
            None
        }
    }

    fn normalize_git_url(url: &str) -> String {
        if url.starts_with("git@github.com:") {
            url.replace("git@github.com:", "https://github.com/")
                .trim_end_matches(".git")
                .to_string()
        } else if url.starts_with("https://github.com/") {
            url.trim_end_matches(".git").to_string()
        } else {
            url.to_string()
        }
    }

    pub fn is_github(&self) -> bool {
        self.remote_url.contains("github.com")
    }

    pub fn generate_file_link(&self, file_path: &str, line: u32) -> Option<String> {
        if !self.is_github() {
            return None;
        }

        let relative_path = self.make_relative_path(file_path)?;

        Some(format!(
            "{}/blob/{}/{}#L{}",
            self.remote_url, self.commit_hash, relative_path, line
        ))
    }

    fn make_relative_path(&self, file_path: &str) -> Option<String> {
        let path = Path::new(file_path);

        if path.is_absolute() {
            path.strip_prefix(&self.root_path)
                .ok()
                .and_then(|p| p.to_str())
                .map(|s| s.to_string())
        } else {
            Some(file_path.to_string())
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    // ── helpers ──────────────────────────────────────────────────────────────

    /// Create a fresh temporary directory and return its handle.
    fn tmp() -> TempDir {
        TempDir::new().expect("failed to create temp dir")
    }

    /// Build a `SearchConfig` with the given timeout in milliseconds.
    fn cfg_ms(ms: u64) -> SearchConfig {
        SearchConfig {
            timeout: Duration::from_millis(ms),
        }
    }

    // ── existing tests ───────────────────────────────────────────────────────

    #[test]
    fn test_normalize_git_url_ssh() {
        let url = "git@github.com:dotandev/hintents.git";
        let normalized = GitRepository::normalize_git_url(url);
        assert_eq!(normalized, "https://github.com/dotandev/hintents");
    }

    #[test]
    fn test_normalize_git_url_https() {
        let url = "https://github.com/dotandev/hintents.git";
        let normalized = GitRepository::normalize_git_url(url);
        assert_eq!(normalized, "https://github.com/dotandev/hintents");
    }

    #[test]
    fn test_is_github() {
        let repo = GitRepository {
            remote_url: "https://github.com/dotandev/hintents".to_string(),
            branch: "main".to_string(),
            commit_hash: "abc123".to_string(),
            root_path: PathBuf::from("/tmp/repo"),
        };
        assert!(repo.is_github());
    }

    #[test]
    fn test_generate_file_link() {
        let repo = GitRepository {
            remote_url: "https://github.com/dotandev/hintents".to_string(),
            branch: "main".to_string(),
            commit_hash: "abc123def456".to_string(),
            root_path: PathBuf::from("/tmp/repo"),
        };

        let link = repo.generate_file_link("src/token.rs", 45);
        assert_eq!(
            link,
            Some(
                "https://github.com/dotandev/hintents/blob/abc123def456/src/token.rs#L45"
                    .to_string()
            )
        );
    }

    // ── new find_git_root tests ────────────────────────────────────────────

    /// `.git` is present in the start directory itself.
    #[test]
    fn test_find_git_root_in_current_dir() {
        let root = tmp();
        fs::create_dir(root.path().join(".git")).unwrap();

        let found = GitRepository::find_git_root(root.path(), &SearchConfig::default());
        assert_eq!(found.as_deref(), Some(root.path()));
    }

    /// `.git` is present two levels above the start directory (nested repo).
    #[test]
    fn test_find_git_root_in_parent() {
        let root = tmp();
        fs::create_dir(root.path().join(".git")).unwrap();

        let nested = root.path().join("a").join("b");
        fs::create_dir_all(&nested).unwrap();

        let found = GitRepository::find_git_root(&nested, &SearchConfig::default());
        assert_eq!(found.as_deref(), Some(root.path()));
    }

    /// No `.git` directory exists anywhere in the tree — should return `None`.
    #[test]
    fn test_find_git_root_no_repo() {
        let root = tmp();
        let deep = root.path().join("x").join("y").join("z");
        fs::create_dir_all(&deep).unwrap();

        // Start from a deeply nested directory with no .git anywhere.
        let found = GitRepository::find_git_root(&deep, &SearchConfig::default());
        assert!(found.is_none());
    }

    /// A zero-millisecond timeout always yields `None` regardless of layout.
    #[test]
    fn test_find_git_root_timeout() {
        let root = tmp();
        fs::create_dir(root.path().join(".git")).unwrap();

        // Immediate deadline — the search must never succeed.
        let found = GitRepository::find_git_root(root.path(), &cfg_ms(0));
        assert!(found.is_none());
    }

    /// A symlinked `.git` directory is detected correctly.
    #[cfg(unix)]
    #[test]
    fn test_find_git_root_symlink() {
        let root = tmp();
        let real_git = root.path().join("actual_git_dir");
        fs::create_dir(&real_git).unwrap();

        let git_link = root.path().join(".git");
        std::os::unix::fs::symlink(&real_git, &git_link).unwrap();

        let found = GitRepository::find_git_root(root.path(), &SearchConfig::default());
        assert_eq!(found.as_deref(), Some(root.path()));
    }
}
