// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//
// You may obtain a copy of the License at
//
//
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

//
// You may obtain a copy of the License at
//
//
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

// Example code that deserializes and serializes the model.
// extern crate serde;
// #[macro_use]
// extern crate serde_derive;
// extern crate serde_json;
//
// use generated_module::simulation-request.schema;
//
// fn main() {
//     let json = r#"{"answer": 42}"#;
//     let model: simulation-request.schema = serde_json::from_str(&json).unwrap();
// }

use serde::{Serialize, Deserialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct SimulationRequestSchema {
    network: Network,

    /// Client-generated unique request identifier
    request_id: String,

    version: String,

    xdr: String,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Network {
    Futurenet,

    Public,

    Testnet,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SimulationResponseSchema {
    error: Option<Error>,

    request_id: String,

    result: Option<Result>,

    success: bool,

    version: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Error {
    code: String,

    message: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Result {
    /// Fee charged in stroops
    fee_charged: String,
}
