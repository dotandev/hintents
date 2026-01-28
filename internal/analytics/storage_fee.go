package analytics

type StorageFeeModel struct {
	FeePerByte uint64 // protocol-defined
}

func CalculateStorageFee(deltaBytes int64, model StorageFeeModel) int64 {
	if deltaBytes <= 0 {
		return 0
	}
	return deltaBytes * int64(model.FeePerByte)
}
