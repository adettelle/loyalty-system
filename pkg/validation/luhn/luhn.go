package luhn

import "strconv"

func CheckLuhn(input string) bool {
	_, err := strconv.Atoi(input)
	if err != nil {
		return false
	}

	resStr := []int{}

	if len(input)%2 == 0 { //количество цифр чётное, значит, начинаем со второй цифры (идекс 1)
		for i, val := range input {
			num, _ := strconv.Atoi(string(val))
			if i%2 == 0 {
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
	}

	var res int
	for _, n := range resStr {
		res += n
	}
	return res%10 == 0
}
