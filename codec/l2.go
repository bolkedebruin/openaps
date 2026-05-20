package codec

// BuildL2Frame wraps body with the canonical
// `FB FB <type> <cmd> <body...> <sum_hi> <sum_lo> FE FE` envelope.
//
// The 16-bit checksum is the byte-wise sum of `type`, `cmd`, and every
// byte of `body`, packed big-endian into two bytes.
func BuildL2Frame(typeByte, cmdByte byte, body []byte) []byte {
	out := make([]byte, 0, 4+len(body)+4)
	out = append(out, 0xFB, 0xFB, typeByte, cmdByte)
	out = append(out, body...)
	var sum uint16
	sum += uint16(typeByte)
	sum += uint16(cmdByte)
	for _, b := range body {
		sum += uint16(b)
	}
	out = append(out, byte(sum>>8), byte(sum&0xFF), 0xFE, 0xFE)
	return out
}
