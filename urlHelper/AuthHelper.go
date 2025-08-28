package urlHelper

import "fmt"

type AuthHelper struct {
	host string
}

func Auth(hostList ...string) *AuthHelper {
	if len(hostList) > 1 {
		panic("only allow zero or one host")
	}
	var authHelper *AuthHelper
	if len(hostList) == 0 {
		authHelper = &AuthHelper{host: ""}
	} else {
		authHelper = &AuthHelper{host: hostList[0]}
	}
	return authHelper
}
func (helper *AuthHelper) Register() string {
	return fmt.Sprintf("%v/register", helper.host)
}
func (helper *AuthHelper) MultipleCreateUser() string {
	return fmt.Sprintf("%v/multipleCreateUser", helper.host)
}
func (helper *AuthHelper) BatchCreateUser() string {
	return fmt.Sprintf("%v/batchCreateUser", helper.host)
}
func (helper *AuthHelper) Login() string {
	return fmt.Sprintf("%v/login", helper.host)
}

func (helper *AuthHelper) QueryUser() string {
	return fmt.Sprintf("%v/queryUser", helper.host)
}

func (helper *AuthHelper) GetNotLoginUser() string {
	return fmt.Sprintf("%v/getNotLoginUser", helper.host)
}
func (helper *AuthHelper) ReInitNotLoginSet() string {
	return fmt.Sprintf("%v/reInitNotLoginSet", helper.host)
}
func (helper *AuthHelper) CometAddr() string {
	return fmt.Sprintf("%v/cometAddr", helper.host)
}
func (helper *AuthHelper) Helloword() string {
	return fmt.Sprintf("%v/helloword", helper.host)
}
