package query

type Page struct {
	Items  []Row
	Total  int
	Offset int
	Limit  int
}

func Paginate(rows []Row, offset, limit int) Page {
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	total := len(rows)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end < offset {
		end = total
	}
	if limit == 0 || end > total {
		end = total
	}
	items := make([]Row, 0, end-offset)
	for _, row := range rows[offset:end] {
		items = append(items, row)
	}
	return Page{Items: items, Total: total, Offset: offset, Limit: limit}
}

func (s *Service) Page(offset, limit int) Page {
	return Paginate(s.rows, offset, limit)
}
