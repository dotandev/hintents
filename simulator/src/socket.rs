// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Unix domain socket streaming for real-time diagnostic event delivery.
//!
//! This module enables the Rust simulator to stream diagnostic events
//! to the Go CLI as they occur, rather than buffering everything until
//! the end. This provides real-time feedback for long-running simulations.

use crate::types::DiagnosticEvent;
use serde::{Deserialize, Serialize};
use std::io::{self, Write};
use std::os::unix::net::UnixStream;

/// Messages sent from Rust simulator to Go CLI over the socket
#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum StreamMessage {
    /// A diagnostic event occurred during simulation
    Event { event: DiagnosticEvent },
    
    /// A log message was generated
    Log { message: String },
    
    /// Budget usage update (sent periodically)
    BudgetUpdate {
        cpu_instructions: u64,
        memory_bytes: u64,
    },
    
    /// Simulation completed successfully
    Complete,
    
    /// Simulation failed with an error
    Error { message: String },
}

/// Handles streaming messages over a Unix domain socket
pub struct SocketStreamer {
    stream: UnixStream,
}

impl SocketStreamer {
    /// Create a new streamer connected to the given socket path
    pub fn connect(socket_path: &str) -> io::Result<Self> {
        let stream = UnixStream::connect(socket_path)?;
        Ok(Self { stream })
    }

    /// Send a message over the socket
    pub fn send(&mut self, message: &StreamMessage) -> io::Result<()> {
        // Serialize message to JSON
        let json = serde_json::to_string(message)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))?;
        
        // Write JSON followed by newline delimiter
        writeln!(self.stream, "{}", json)?;
        
        // Flush to ensure immediate delivery
        self.stream.flush()?;
        
        Ok(())
    }

    /// Send a diagnostic event
    pub fn send_event(&mut self, event: DiagnosticEvent) -> io::Result<()> {
        self.send(&StreamMessage::Event { event })
    }

    /// Send a log message
    pub fn send_log(&mut self, message: String) -> io::Result<()> {
        self.send(&StreamMessage::Log { message })
    }

    /// Send a budget update
    pub fn send_budget_update(&mut self, cpu: u64, mem: u64) -> io::Result<()> {
        self.send(&StreamMessage::BudgetUpdate {
            cpu_instructions: cpu,
            memory_bytes: mem,
        })
    }

    /// Send completion signal
    pub fn send_complete(&mut self) -> io::Result<()> {
        self.send(&StreamMessage::Complete)
    }

    /// Send error signal
    pub fn send_error(&mut self, message: String) -> io::Result<()> {
        self.send(&StreamMessage::Error { message })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_stream_message_serialization() {
        let event = DiagnosticEvent {
            event_type: "contract".to_string(),
            contract_id: Some("CABC123".to_string()),
            topics: vec!["transfer".to_string()],
            data: "100".to_string(),
            in_successful_contract_call: true,
        };

        let msg = StreamMessage::Event { event };
        let json = serde_json::to_string(&msg).unwrap();
        
        assert!(json.contains("\"type\":\"event\""));
        assert!(json.contains("contract"));
    }

    #[test]
    fn test_budget_update_serialization() {
        let msg = StreamMessage::BudgetUpdate {
            cpu_instructions: 1000,
            memory_bytes: 2048,
        };
        
        let json = serde_json::to_string(&msg).unwrap();
        assert!(json.contains("\"type\":\"budget_update\""));
        assert!(json.contains("1000"));
    }
}
