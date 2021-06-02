package db

type User struct {
	User_Id      string  `json:"user_id" bson:"user_id"`
	Display_Name string  `json:"display_name" bson:"display_name"`
	Points       float32 `json:"points" bson:"points"`
	Rank         int     `json:"rank" bson:"rank"`
	Country      string  `json:"country,omitempty" bson:"country,omitempty"`
}

type LeaderBoardRespond struct {
	Rank         int     `json:"rank" bson:"rank"`
	Points       float32 `json:"points" bson:"points"`
	Display_Name string  `json:"display_name" bson:"display_name"`
	Country      string  `json:"country" bson:"country"`
}

type Score struct {
	Score_worth float32 `json:"score_worth"`
	User_Id     string  `json:"user_id"`
	Timestamp   int64   `json:"timestamp"`
}
