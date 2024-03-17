package luna

type signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

type unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type integer interface {
	signed | unsigned
}

// ordered is a constraint that permits only integer and string types.
type ordered interface {
	integer | ~string
}
