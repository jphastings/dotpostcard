package atproto

// FollowRecordType is the value of a follow record's "$type" field.
const FollowRecordType = "org.dotpostcard.follow"

// Follow is an org.dotpostcard.follow record: its author follows the collection at Subject.
type Follow struct {
	Type      string `json:"$type"`
	Subject   string `json:"subject"`
	CreatedAt string `json:"createdAt"`
}
