package urlHelper

import "fmt"

type CometHelper struct {
	host string
}

func Comet(hostList ...string) *CometHelper {
	if len(hostList) > 1 {
		panic("only allow zero or one host")
	}
	var cometHelper *CometHelper
	if len(hostList) == 0 {
		cometHelper = &CometHelper{host: ""}
	} else {
		cometHelper = &CometHelper{host: hostList[0]}
	}
	return cometHelper
}
func (helper *CometHelper) Connect() string {
	return fmt.Sprintf("%v/connect", helper.host)
}
