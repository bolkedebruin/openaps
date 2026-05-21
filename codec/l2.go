package codec

// BuildL2Frame wraps body with the canonical
// `FB FB <inner_len> <cmd> <body...> <sum_hi> <sum_lo> FE FE` envelope.
//
// inner_len is the count of cmd + body bytes that follow (1 + len(body));
// it sits at offset 2 of the frame. The 16-bit checksum is the byte-wise
// sum of inner_len, cmd, and every byte of body, packed big-endian into
// two bytes.
func BuildL2Frame(cmdByte byte, body []byte) []byte {
	innerLen := byte(1 + len(body))
	out := make([]byte, 0, 4+len(body)+4)
	out = append(out, L2SOF1, L2SOF2, innerLen, cmdByte)
	out = append(out, body...)
	sum := uint16(innerLen) + uint16(cmdByte)
	for _, b := range body {
		sum += uint16(b)
	}
	out = append(out, byte(sum>>8), byte(sum&0xFF), L2EOF1, L2EOF2)
	return out
}
