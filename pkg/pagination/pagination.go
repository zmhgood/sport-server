package pagination

// Normalize keeps list limits bounded and prevents (page-1)*size overflow.
func Normalize(page, size, fallback int) (int, int) {
	if size < 1 || size > 100 {
		size = fallback
	}
	if page < 1 {
		page = 1
	}
	maxPage := int(^uint(0)>>1) / size
	if page > maxPage {
		page = maxPage
	}
	return page, size
}

// Bounds checks the page before multiplication; out-of-range pages are empty.
func Bounds(page, size, total int) (int, int) {
	page, size = Normalize(page, size, 10)
	if total <= 0 || page-1 > (total-1)/size {
		return total, total
	}
	start := (page - 1) * size
	end := total
	if total-start > size {
		end = start + size
	}
	return start, end
}
