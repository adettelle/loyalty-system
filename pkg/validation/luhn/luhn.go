package luhn

import "strconv"

func CheckLuhn(input string) bool {
	_, err := strconv.Atoi(input)
	if err != nil {
		return false
	}

	resStr := []int{}

	startFrom := 0
	if len(input)%2 == 0 {
		startFrom = 1
	}

	// если количество цифр чётное, значит, начинаем со второй цифры (идекс 1)
	// если нечётное, начинаем спервой цифры (идекс 0)
	for i, val := range input[startFrom:] {
		num, _ := strconv.Atoi(string(val))
		if i%2 != 0 {
			x := num * 2
			if x > 9 {
				x = x - 9
				resStr = append(resStr, x)
			} else {
				resStr = append(resStr, x)
			}
		} else {
			resStr = append(resStr, num)
		}
	}

	var res int
	for _, n := range resStr {
		res += n
	}
	return res%10 == 0
}
