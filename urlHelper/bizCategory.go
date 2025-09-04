package urlHelper

import "fmt"

// 需要和应用名称一致，包括大小写
const CONST_BUSINESS_AUTH = "auth"
const CONST_BUSINESS_COMET = "comet"

const State_Auth_MySQL = "/appState/auth/mysql"
const State_IM_Redis = "/appState/im/redis"

const State_Friend_MySQL = "/appState/auth/mysql"

const State_Auth_http = "/appState/auth/http"
const State_Auth_grpc = "/appState/auth/grpc"

const State_Comet_http = "/appState/comet/http"
const State_Comet_grpc = "/appState/comet/grpc"

type MessageCategory uint32

const (
	Category_AUTH   MessageCategory = (1 + iota) << 8 // appCategory/1
	Category_Comet                                    // appCategory/2
	Category_Friend                                   // appCategory/3
	Category_Chat                                     // appCategory/4

)

func Category(category MessageCategory) string {
	return fmt.Sprintf("/appCategory/%v", category>>8)
}
