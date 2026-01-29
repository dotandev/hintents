use std::path::PathBuf;

pub fn theme_path() -> PathBuf {
    let mut path = std::env::var("HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|_| PathBuf::from("."));
    path.push(".erst");
    path.push("theme.json");
    path
}
