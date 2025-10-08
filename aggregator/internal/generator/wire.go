package generator

import "github.com/google/wire"

// ProviderSet содержит провайдеры источников данных генератора для интеграции с wire.
var ProviderSet = wire.NewSet(NewRandomSource)
