package dto

type ListEmployeesQuery struct {
	Email     string `form:"email"`
	FirstName string `form:"first_name"`
	LastName  string `form:"last_name"`
	Position  string `form:"position"`
	Page      int    `form:"page" binding:"min=1"`
	PageSize  int    `form:"page_size" binding:"min=1,max=100"`
}
