pub struct GistUploader {
    token: String,
}

impl GistUploader {
    pub fn new(token: String) -> Self {
        Self { token }
    }
}

impl TraceUploader for GistUploader {
    fn upload(...) -> Result<String, AppError> {
        // build request
        // send HTTP
        // parse URL
        // return link
    }
}
