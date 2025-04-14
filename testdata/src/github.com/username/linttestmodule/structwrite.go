package linttestmodule

type NonWritable struct {
	A int
}

func NewNonWritable() NonWritable {
	return NonWritable{
		A: 1,
	}
}

// multiple words
//command
// hello
func (nw *NonWritable) SetA() {
	nw.A = 1 // want "write to NonWritable field outside constructor"
}

func NonWritableSetALiteral() {
	nw := NewNonWritable()
	nw.A = 1 // want "write to NonWritable field outside constructor"
}

func NonWritableSetADoublePtr() {
	nw := NewNonWritable()
	nwp := &nw
	nwpp := &nwp
	(*nwpp).A = 1 // want "write to NonWritable field outside constructor"
}

type Writable struct {
	A int
}

func NewWritable() Writable {
	return Writable{
		A: 1,
	}
}

func (w Writable) SetA() {
	w.A = 1
}

type EmbedsNonWritable struct {
	NonWritable
}

func NewEmbedsNonWritable() EmbedsNonWritable {
	return EmbedsNonWritable{
		NonWritable: NewNonWritable(),
	}
}

func (w *EmbedsNonWritable) SetA() {
	w.A = 1 // want "write to NonWritable field outside constructor"
}
