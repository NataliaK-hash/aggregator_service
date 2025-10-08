package wire

type ProviderSet struct{}

func Build(_ ...any) ProviderSet {
	return ProviderSet{}
}

func NewSet(_ ...any) ProviderSet {
	return ProviderSet{}
}
