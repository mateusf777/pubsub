package domain

func Join(byteSlices ...[]byte) []byte {

	n := 0
	for _, bs := range byteSlices {
		n += len(bs)
	}

	result := make([]byte, n)
	bp := copy(result, byteSlices[0])
	for _, v := range byteSlices[1:] {
		bp += copy(result[bp:], v)
	}

	return result
}
