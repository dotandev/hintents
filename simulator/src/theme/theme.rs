#[derive(Debug, Clone)]
pub struct Theme {
    pub span: String,
    pub event: String,
    pub error: String,
    pub warning: String,
    pub info: String,
    pub dim: String,
    pub highlight: String,
}

impl Default for Theme {
    fn default() -> Self {
        Self {
            span: "blue".into(),
            event: "cyan".into(),
            error: "red".into(),
            warning: "yellow".into(),
            info: "green".into(),
            dim: "bright_black".into(),
            highlight: "magenta".into(),
        }
    }
}
