package prng

type PCG32 struct {
	state uint64
	inc   uint64
}

func NewPCG32(seed uint64) *PCG32 {
	return &PCG32{
		state: 0,
		inc:   seed | 1,
	}
}

func (p *PCG32) Next() uint32 {
	oldstate := p.state
	p.state = oldstate*6364136223846793005 + p.inc
	xorshifted := uint32(((oldstate >> 18) ^ oldstate) >> 27)
	rot := uint32(oldstate >> 59)
	return (xorshifted >> rot) | (xorshifted << ((-rot) & 31))
}

func (p *PCG32) NextFloat32() float32 {
	return float32(p.Next()) / (1 << 32)
}
