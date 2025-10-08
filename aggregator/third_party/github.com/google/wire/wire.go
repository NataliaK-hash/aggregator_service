package wire

type ProviderSet struct{}

func NewSet(_ ...interface{}) ProviderSet {
	return ProviderSet{}
}

func Build(_ ...interface{}) ProviderSet {
	return ProviderSet{}
}
