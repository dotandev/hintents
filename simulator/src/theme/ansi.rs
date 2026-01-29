use colored::Colorize;

pub fn apply(color: &str, text: &str) -> String {
    match color {
        "black" => text.black().to_string(),
        "red" => text.red().to_string(),
        "green" => text.green().to_string(),
        "yellow" => text.yellow().to_string(),
        "blue" => text.blue().to_string(),
        "magenta" => text.magenta().to_string(),
        "cyan" => text.cyan().to_string(),
        "white" => text.white().to_string(),
        "bright_black" => text.bright_black().to_string(),
        "bright_red" => text.bright_red().to_string(),
        "bright_blue" => text.bright_blue().to_string(),
        _ => text.normal().to_string(), // safety fallback
    }
}
