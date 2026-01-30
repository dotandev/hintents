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

package localization

var EnglishMessages = map[string]string{
	"cli.debug.short":             "Debug a failed Soroban transaction",
	"cli.debug.long":              "Fetch and prepare a transaction for simulation",
	"cli.debug.example.basic":     "erst debug <tx-hash>",
	"cli.debug.example.testnet":   "erst debug --network testnet <tx-hash>",
	"cli.debug.example.gas_model": "erst debug --gas-model ./custom-gas-model.json <tx-hash>",
	"cli.debug.flag.network":      "Stellar network to use (testnet, mainnet, futurenet)",
	"cli.debug.flag.rpc_url":      "Custom Horizon RPC URL",
	"cli.debug.flag.gas_model":    "Path to custom gas model JSON file",

	"cli.auth_debug.short":            "Debug multi-signature and threshold-based authorization failures",
	"cli.auth_debug.long":             "Analyze multi-signature authorization flows and identify failures",
	"cli.auth_debug.flag.detailed":    "Show detailed analysis and missing signatures",
	"cli.auth_debug.flag.json":        "Output as JSON",
	"cli.auth_debug.flag.custom_auth": "Path to custom auth configuration JSON",

	"error.invalid_network":       "invalid network: %s",
	"error.network_required":      "network must be one of: testnet, mainnet, futurenet",
	"error.fetch_transaction":     "failed to fetch transaction: %w",
	"error.parse_gas_model":       "failed to parse gas model: %w",
	"error.gas_model_validation":  "gas model validation failed: %s",
	"error.invalid_rpc_url":       "invalid RPC URL: %s",
	"error.transaction_not_found": "transaction not found: %s",

	"info.fetching_transaction":  "Fetching transaction for simulation",
	"info.gas_model_loaded":      "Gas model loaded and validated",
	"info.auth_analysis_started": "Fetching transaction for auth analysis",

	"output.transaction_envelope":    "Transaction Envelope: %d bytes",
	"output.custom_gas_model":        "Custom Gas Model Applied:",
	"output.network":                 "Network: %s",
	"output.total_costs":             "Total Costs: %d",
	"output.resource_limits":         "Resource Limits configured",
	"output.authorization_failed":    "✗ Authorization FAILED",
	"output.authorization_succeeded": "✓ Authorization SUCCEEDED",
	"output.summary_metrics":         "--- SUMMARY METRICS ---",
	"output.missing_signatures":      "--- MISSING SIGNATURES ---",
	"output.required_weight":         "required weight: %d",

	"validation.model_required":   "gas model file path cannot be empty",
	"validation.model_file_read":  "failed to read gas model file: %w",
	"validation.json_parse_error": "failed to parse gas model JSON: %w",
}

var SpanishMessages = map[string]string{
	"cli.debug.short":             "Depurar una transacción Soroban fallida",
	"cli.debug.long":              "Obtener y preparar una transacción para simulación",
	"cli.debug.example.basic":     "erst debug <tx-hash>",
	"cli.debug.example.testnet":   "erst debug --network testnet <tx-hash>",
	"cli.debug.example.gas_model": "erst debug --gas-model ./modelo-gas-personalizado.json <tx-hash>",
	"cli.debug.flag.network":      "Red Stellar a utilizar (testnet, mainnet, futurenet)",
	"cli.debug.flag.rpc_url":      "URL de RPC personalizada",
	"cli.debug.flag.gas_model":    "Ruta al archivo JSON del modelo de gas personalizado",

	"cli.auth_debug.short":            "Depurar fallos de autorización con múltiples firmas",
	"cli.auth_debug.long":             "Analizar flujos de autorización con múltiples firmas",
	"cli.auth_debug.flag.detailed":    "Mostrar análisis detallado y firmas faltantes",
	"cli.auth_debug.flag.json":        "Salida como JSON",
	"cli.auth_debug.flag.custom_auth": "Ruta a archivo de configuración de autenticación",

	"error.invalid_network":       "red inválida: %s",
	"error.network_required":      "la red debe ser una de: testnet, mainnet, futurenet",
	"error.fetch_transaction":     "error al obtener transacción: %w",
	"error.parse_gas_model":       "error al analizar modelo de gas: %w",
	"error.gas_model_validation":  "validación de modelo de gas fallida: %s",
	"error.invalid_rpc_url":       "URL de RPC inválida: %s",
	"error.transaction_not_found": "transacción no encontrada: %s",

	"info.fetching_transaction":  "Obteniendo transacción para simulación",
	"info.gas_model_loaded":      "Modelo de gas cargado y validado",
	"info.auth_analysis_started": "Obteniendo transacción para análisis de autorización",

	"output.transaction_envelope":    "Envolvente de Transacción: %d bytes",
	"output.custom_gas_model":        "Modelo de Gas Personalizado Aplicado:",
	"output.network":                 "Red: %s",
	"output.total_costs":             "Costos Totales: %d",
	"output.resource_limits":         "Límites de Recursos configurados",
	"output.authorization_failed":    "✗ Autorización FALLIDA",
	"output.authorization_succeeded": "✓ Autorización EXITOSA",
	"output.summary_metrics":         "--- MÉTRICAS DE RESUMEN ---",
	"output.missing_signatures":      "--- FIRMAS FALTANTES ---",
	"output.required_weight":         "peso requerido: %d",

	"validation.model_required":   "la ruta del archivo de modelo de gas no puede estar vacía",
	"validation.model_file_read":  "error al leer archivo de modelo de gas: %w",
	"validation.json_parse_error": "error al analizar JSON del modelo de gas: %w",
}

var ChineseMessages = map[string]string{
	"cli.debug.short":             "调试失败的 Soroban 交易",
	"cli.debug.long":              "获取并准备用于模拟的交易",
	"cli.debug.example.basic":     "erst debug <tx-hash>",
	"cli.debug.example.testnet":   "erst debug --network testnet <tx-hash>",
	"cli.debug.example.gas_model": "erst debug --gas-model ./custom-gas-model.json <tx-hash>",
	"cli.debug.flag.network":      "使用的 Stellar 网络 (testnet, mainnet, futurenet)",
	"cli.debug.flag.rpc_url":      "自定义 Horizon RPC URL",
	"cli.debug.flag.gas_model":    "自定义 gas 模型 JSON 文件路径",

	"cli.auth_debug.short":            "调试多签名和阈值授权失败",
	"cli.auth_debug.long":             "分析多签名授权流程并识别失败",
	"cli.auth_debug.flag.detailed":    "显示详细分析和缺失签名",
	"cli.auth_debug.flag.json":        "输出为 JSON 格式",
	"cli.auth_debug.flag.custom_auth": "自定义身份验证配置 JSON 文件路径",

	"error.invalid_network":       "无效的网络: %s",
	"error.network_required":      "网络必须是以下之一: testnet, mainnet, futurenet",
	"error.fetch_transaction":     "获取交易失败: %w",
	"error.parse_gas_model":       "解析 gas 模型失败: %w",
	"error.gas_model_validation":  "gas 模型验证失败: %s",
	"error.invalid_rpc_url":       "无效的 RPC URL: %s",
	"error.transaction_not_found": "交易未找到: %s",

	"info.fetching_transaction":  "正在获取用于模拟的交易",
	"info.gas_model_loaded":      "Gas 模型已加载并验证",
	"info.auth_analysis_started": "正在获取用于授权分析的交易",

	"output.transaction_envelope":    "交易包: %d 字节",
	"output.custom_gas_model":        "自定义 Gas 模型已应用:",
	"output.network":                 "网络: %s",
	"output.total_costs":             "总成本: %d",
	"output.resource_limits":         "资源限制已配置",
	"output.authorization_failed":    "✗ 授权失败",
	"output.authorization_succeeded": "✓ 授权成功",
	"output.summary_metrics":         "--- 摘要指标 ---",
	"output.missing_signatures":      "--- 缺失签名 ---",
	"output.required_weight":         "所需权重: %d",

	"validation.model_required":   "gas 模型文件路径不能为空",
	"validation.model_file_read":  "读取 gas 模型文件失败: %w",
	"validation.json_parse_error": "解析 gas 模型 JSON 失败: %w",
}

func LoadTranslations() error {
	if err := RegisterMessages(English, EnglishMessages); err != nil {
		return err
	}
	if err := RegisterMessages(Spanish, SpanishMessages); err != nil {
		return err
	}
	if err := RegisterMessages(Chinese, ChineseMessages); err != nil {
		return err
	}
	return nil
}
