package spki

const vo = "aeiouy"
const co = "bcdfghklmnprstvzx"

func Babbleprint(p []byte) string {
	buf := make([]byte, (((len(p)+1)/2)+1)*6-1)

	var a, b, c, d, e byte
	i := 0
	k := 1
	check := byte(1)
	buf[0] = co[16]
	for i < len(p)-1 {
		a = (((p[i] >> 6) & 3) + check) % 6
		b = (p[i] >> 2) & 15
		c = ((p[i] & 3) + (check / 6)) % 6
		d = (p[i+1] >> 4) & 15
		e = p[i+1] & 15

		check = (check*5 + p[i]*7 + p[i+1]) % 36

		buf[k+0] = vo[a]
		buf[k+1] = co[b]
		buf[k+2] = vo[c]
		buf[k+3] = co[d]
		buf[k+4] = '-'
		buf[k+5] = co[e]

		i += 2
		k += 6
	}

	if (len(p) % 2) != 0 {
		a = (((p[i] >> 6) & 3) + check) % 6
		b = (p[i] >> 2) & 15
		c = ((p[i] & 3) + (check / 6)) % 6
	} else {
		a = check % 6
		b = 16
		c = check / 6
	}

	buf[k+0] = vo[a]
	buf[k+1] = co[b]
	buf[k+2] = vo[c]
	buf[k+3] = co[16]
	return string(buf)
}
