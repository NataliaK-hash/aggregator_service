package wire

type ProviderSet struct{}

type Interface interface{}

func NewSet(...any) ProviderSet { return ProviderSet{} }

func Build(...any) {}

func Struct(any, ...string) ProviderSet { return ProviderSet{} }

func Bind(any, any) {}
