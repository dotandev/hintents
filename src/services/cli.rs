use clap::Parser;

#[derive(Parser, Debug)]
pub struct Cli {
    #[arg(long)]
    pub share: bool,

    #[arg(long)]
    pub public: bool,
}
