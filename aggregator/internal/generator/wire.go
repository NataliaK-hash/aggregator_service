package generator

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewRandomSource)
