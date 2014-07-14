package endian
import "io"

func LoadU16BE(buf []byte) uint16 {
  return (uint16(buf[0]) << 8) | uint16(buf[1])
}

func StoreU16BE(x uint16, buf []byte) {
  buf[0] = byte((x & 0xFF00) >> 8)
  buf[1] = byte(x & 0x00FF)
}

func LoadU32BE(buf []byte) uint32 {
  return (uint32(buf[0]) << 24) | (uint32(buf[1]) << 16) | (uint32(buf[2]) << 8) | (uint32(buf[3]))
}

func EncodeU16BE(x uint16) [2]byte {
  return [2]byte { byte((x & 0xFF00) >> 8), byte(x & 0x00FF) }
}

func EncodeU32BE(x uint32) [4]byte {
  return [4]byte { byte((x & 0xFF000000) >> 24), byte((x & 0x00FF0000) >>  16),
                   byte((x & 0x0000FF00) >>  8), byte((x & 0x000000FF)) }
}

func WriteU16BE(w io.Writer, x uint16) (int, error) {
  b := EncodeU16BE(x)
  return w.Write(b[:])
}

func WriteU32BE(w io.Writer, x uint32) (int, error) {
  b := EncodeU32BE(x)
  return w.Write(b[:])
}
