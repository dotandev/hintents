pub trait TraceUploader {
    fn upload(&self, content: &str, public: bool) -> Result<String, AppError>;
}
