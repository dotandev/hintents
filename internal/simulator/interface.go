package simulator

type Runner interface {
	Run(req *SimulationRequest) (*SimulationResponse, error)
}
