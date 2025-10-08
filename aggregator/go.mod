module aggregator

go 1.24

require (
    github.com/google/uuid v0.0.0
    github.com/google/wire v0.6.0
)

replace (
    github.com/google/uuid => ./internal/uuidstub
    github.com/google/wire => ./internal/wirestub
)

