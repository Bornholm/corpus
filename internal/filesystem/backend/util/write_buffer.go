package util

type WriteBuffer struct {
	d   []byte
	off int64
}

func NewWriteBuffer() *WriteBuffer {
	return &WriteBuffer{make([]byte, 0), 0}
}

// Bytes returns the WriteBuffer's underlying data. This value will remain valid so long
// as no other methods are called on the WriteBuffer.
func (wb *WriteBuffer) Bytes() []byte {
	return wb.d
}

func (wb *WriteBuffer) WriteAt(dat []byte, off int64) (int, error) {
	if int(off) == len(wb.d) {
		wb.d = append(wb.d, dat...)
		return len(dat), nil
	}

	if int(off)+len(dat) >= len(wb.d) {
		nd := make([]byte, int(off)+len(dat))
		copy(nd, wb.d)
		wb.d = nd
	}

	copy(wb.d[int(off):], dat)
	return len(dat), nil
}

func (wb *WriteBuffer) Write(dat []byte) (int, error) {
	n, err := wb.WriteAt(dat, wb.off)
	wb.off += int64(n)
	return n, err
}
