package wire

type ProviderSet struct{}

func Build(_ ...any) ProviderSet {
	return ProviderSet{}
}
