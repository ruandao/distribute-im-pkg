package urlHelper

import "fmt"

type FriendHelper struct {
	host string
}

func Friend(hostList ...string) *FriendHelper {
	if len(hostList) > 1 {
		panic("only allow zero or one host")
	}
	var friendHelper *FriendHelper
	if len(hostList) == 0 {
		friendHelper = &FriendHelper{host: ""}
	} else {
		friendHelper = &FriendHelper{host: hostList[0]}
	}
	return friendHelper
}
func (helper *FriendHelper) AddFriendsForAllUser() string {
	return fmt.Sprintf("%v/AddFriendsForAllUser", helper.host)
}
func (helper *FriendHelper) AverageFriendCnt() string {
	return fmt.Sprintf("%v/AverageFriendCnt", helper.host)
}
func (helper *FriendHelper) FriendStatusUpdate() string {
	return fmt.Sprintf("%v/FriendStatusUpdate", helper.host)
}

func (helper *FriendHelper) FriendDelete() string {
	return fmt.Sprintf("%v/FriendDelete", helper.host)
}

func (helper *FriendHelper) FriendListGet() string {
	return fmt.Sprintf("%v/FriendListGet", helper.host)
}

func (helper *FriendHelper) FriendVersionDiffer() string {
	return fmt.Sprintf("%v/FriendVersionDiffer", helper.host)
}
func (helper *FriendHelper) FriendLastVerion() string {
	return fmt.Sprintf("%v/FriendLastVersion", helper.host)
}
