package alloc

import "errors"

const (
	minSupportedDigits = 6
	maxSupportedDigits = 12

	legacy9DigitMin int64 = 100_000_000
	legacy9DigitMax int64 = 999_999_999
	legacy9DigitMod int64 = legacy9DigitMax - legacy9DigitMin + 1
)

// 9 位 UIN 保持历史映射，避免已发出去的号码失真。
const legacy9DigitMul int64 = 626262629

// legacy9DigitMul 在 9 位区间模空间中的乘法逆元。
const legacy9DigitMulInv int64 = 146863469

type uinMapping struct {
	min    int64
	max    int64
	mod    int64
	mul    int64
	mulInv int64
	offset int64
}

// Obfuscate 内部有序 seq 转换为对外乱序 UIN，支持 6-12 位。
func Obfuscate(seq int64) (int64, error) {
	mapping, err := mappingForValue(seq)
	if err != nil {
		return 0, err
	}
	x := seq - mapping.min
	y := (mulMod(x, mapping.mul, mapping.mod) + mapping.offset) % mapping.mod
	return y + mapping.min, nil
}

// DeObfuscate 对外 UIN 还原内部原始 seq，支持 6-12 位。
func DeObfuscate(uin int64) (int64, error) {
	mapping, err := mappingForValue(uin)
	if err != nil {
		return 0, err
	}
	y := uin - mapping.min - mapping.offset
	y %= mapping.mod
	if y < 0 {
		y += mapping.mod
	}
	x := mulMod(y, mapping.mulInv, mapping.mod)
	return x + mapping.min, nil
}

func ValidateUINRange(start, end int64) error {
	if start <= 0 || end <= 0 {
		return errors.New("uin range must be positive")
	}
	if start > end {
		return errors.New("uin range start greater than end")
	}
	if _, err := mappingForValue(start); err != nil {
		return err
	}
	if _, err := mappingForValue(end); err != nil {
		return err
	}
	return nil
}

func mappingForValue(value int64) (uinMapping, error) {
	if value <= 0 {
		return uinMapping{}, errors.New("uin must be positive")
	}
	digits := decimalDigits(value)
	if digits < minSupportedDigits || digits > maxSupportedDigits {
		return uinMapping{}, errors.New("uin out of supported 6-12 digit range")
	}
	return mappingForDigits(digits)
}

func mappingForDigits(digits int) (uinMapping, error) {
	min := pow10(digits - 1)
	max := pow10(digits) - 1
	mod := max - min + 1

	if digits == 9 {
		return uinMapping{
			min:    legacy9DigitMin,
			max:    legacy9DigitMax,
			mod:    legacy9DigitMod,
			mul:    legacy9DigitMul,
			mulInv: legacy9DigitMulInv,
			offset: 0,
		}, nil
	}

	mul := multiplierForDigits(digits)
	mulInv, err := modInverse(mul, mod)
	if err != nil {
		return uinMapping{}, err
	}
	offset := offsetForDigits(digits) % mod

	return uinMapping{
		min:    min,
		max:    max,
		mod:    mod,
		mul:    mul,
		mulInv: mulInv,
		offset: offset,
	}, nil
}

func decimalDigits(value int64) int {
	digits := 0
	for value > 0 {
		digits++
		value /= 10
	}
	return digits
}

func pow10(exp int) int64 {
	result := int64(1)
	for i := 0; i < exp; i++ {
		result *= 10
	}
	return result
}

func multiplierForDigits(digits int) int64 {
	switch digits {
	case 6:
		return 700_001
	case 7:
		return 7_000_001
	case 8:
		return 70_000_001
	case 10:
		return 7_000_000_001
	case 11:
		return 70_000_000_001
	case 12:
		return 700_000_000_001
	default:
		return 7
	}
}

func offsetForDigits(digits int) int64 {
	switch digits {
	case 6:
		return 128_731
	case 7:
		return 3_579_123
	case 8:
		return 48_392_017
	case 10:
		return 2_750_193_841
	case 11:
		return 63_015_498_217
	case 12:
		return 481_230_567_891
	default:
		return 0
	}
}

func modInverse(a, m int64) (int64, error) {
	gcd, x, _ := extendedGCD(a, m)
	if gcd != 1 {
		return 0, errors.New("multiplier is not invertible under current range")
	}
	x %= m
	if x < 0 {
		x += m
	}
	return x, nil
}

func extendedGCD(a, b int64) (gcd, x, y int64) {
	if b == 0 {
		return a, 1, 0
	}
	gcd, x1, y1 := extendedGCD(b, a%b)
	return gcd, y1, x1 - (a/b)*y1
}

// mulMod 防止 int64 乘法溢出：(a*b) mod m
func mulMod(a, b, m int64) int64 {
	var res int64
	a %= m
	for b > 0 {
		if b&1 == 1 {
			res = (res + a) % m
		}
		a = (a * 2) % m
		b >>= 1
	}
	return res
}
