// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use std::path::{Path, PathBuf};
use std::process::Command;

#[derive(Debug, Clone)]
pub struct GitRepository {
    pub remote_url: String,
    pub branch: String,
    pub commit_hash: String,
    pub root_path: PathBuf,
}

impl GitRepository {
    pub fn detect(start_path: &Path) -> Option<Self> {
        let root_path = Self::find_git_root(start_path)?;
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

    fn find_git_root(start_path: &Path) -> Option<PathBuf> {
        let mut current = start_path.to_path_buf();
        
        loop {
            let git_dir = current.join(".git");
            if git_dir.exists() {
                return Some(current);
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
            self.remote_url,
            self.commit_hash,
            relative_path,
            line
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
            Some("https://github.com/dotandev/hintents/blob/abc123def456/src/token.rs#L45".to_string())
        );
    }
}
