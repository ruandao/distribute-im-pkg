package cache

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ruandao/distribute-im-pkg/config/appConfigLib"
	"github.com/ruandao/distribute-im-pkg/lib"
	"github.com/ruandao/distribute-im-pkg/lib/db/rdb"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/randx"
	"github.com/ruandao/distribute-im-pkg/lib/runner"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"

	"github.com/go-redis/redis/v8"
)

// warnning, 如果要批量操作的话,需要使用hash、zset 之类的,要不然不同的key会落在不同的实例上
const (
	keyLoginIdPrefix_uuid               = "c_%v"           // 登录ID的格式, 用户登录后,会把用户数据缓存到 c_${uuid} 中, 过期时间为 loginSession, uuid 会下发给用户
	keyLoginIdPrefix                    = "c_"             // 登录ID的格式前缀
	keyCometEndpointPrefix_userId       = "comet_%v"       // 存储用户长连接网关节点的地址
	keyRouteTagPrefix_Id                = "routeTag_%v"    // 路由表, 存储ID的路由信息, 这个ID 可能是用户ID、SessionID、订单ID、等
	keyId2usernameMapPrefix_tmpStoreKey = "id2username_%v" // 初始化 userId->用户名映射的hash
	keyId2usernameMap                   = "id2username"    // 存放 userId 到用户名的映射的hash, 用于反查用户ID到用户名、遍历userId
	keyUsername2IdMapPrefix_tmpStoreKey = "username2id_%v" // 初始化 userId->用户名映射的hash
	keyUsername2IdMap                   = "username2id"    // 存放 userId 到用户名的映射的hash, 用于反查用户ID到用户名、遍历userId
	keyLoginStatusPrefix_tmpStoreKey    = "loginStatus_%v" // 初始化用户列表时临时用来存放 uid: 最后登录时间 的 zset
	keyLoginStatus                      = "loginStatus"    // 用来存放 uid: 最后登录时间 的 zset
	keyTableCnt                         = "tableCnt"
	keyTableCntPrefix_tmpStoreKey       = "tableCnt_%v" // 用来临时存储 tableCnt
)

type Cache struct {
}

func NewCache() *Cache {
	cache := &Cache{}
	return cache
}

func Get() *Cache {
	return NewCache()
}

func (cache *Cache) Login(ctx context.Context, uid lib.Int64S, username string, userData string) (string, error) {
	loginIdI := lib.GetUuid()
	loginIdS := fmt.Sprintf(keyLoginIdPrefix_uuid, loginIdI)

	loginSession := appConfigLib.GetAppConfig().BConfig.Get("loginSession").(int)
	expireDuration := time.Duration(loginSession) * time.Second

	err := rdb.GetRedisC().Set(ctx, loginIdS, userData, expireDuration)
	if err != nil {
		return "", xerr.NewXError(err, "登录用户失败")
	}

	userIdLoginTimeMap := make(map[string]float64)
	userIdLoginTimeMap[uid.ToString()] = float64(time.Now().Unix())
	idUserNameMap := make(map[string]string)
	idUserNameMap[uid.ToString()] = username
	err = cache.UpdateLoginStatus(ctx, "", userIdLoginTimeMap)
	if err != nil {
		return "", xerr.NewXError(err, fmt.Sprintf("将用户登录时间标记为'现在'失败,受影响用户有: %v", uid))
	}
	err = cache.PutId2UserName(ctx, "", idUserNameMap)
	if err != nil {
		return "", xerr.NewXError(err, fmt.Sprintf("更新ID到用户名的映射失败: %v", idUserNameMap))
	}

	return loginIdS, nil
}
func (cache *Cache) Logout(ctx context.Context, logindId string) error {
	err := rdb.GetRedisC().Del(ctx, fmt.Sprintf(keyLoginIdPrefix_uuid, logindId))
	if err != nil {
		return xerr.NewXError(err, "登出用户失败")
	}
	return nil
}

func (cache *Cache) _cometEndpoint(uid lib.Int64S) string {
	return fmt.Sprintf(keyCometEndpointPrefix_userId, uid.ToString())
}

func (cache *Cache) ConnExtendExpire(ctx context.Context, uid lib.Int64S) error {
	cometKeepAliveTimeout := appConfigLib.GetAppConfig().BConfig.Get("cometKeepAliveTimeout").(int)
	err := rdb.GetRedisC().Expire(ctx, cache._cometEndpoint(uid), time.Duration(cometKeepAliveTimeout)*time.Second)
	if err != nil {
		return xerr.NewXError(err, "长连接端点延期失败")
	}
	return nil
}

func (cache *Cache) ConnPutEndpointNX(ctx context.Context, uid lib.Int64S, host string) error {
	cometKeepAliveTimeout := appConfigLib.GetAppConfig().BConfig.Get("cometKeepAliveTimeout").(int)
	updateSuccess, err := rdb.GetRedisC().SetNX(ctx, cache._cometEndpoint(uid), host, time.Duration(cometKeepAliveTimeout)*time.Second)
	if err != nil {
		return xerr.NewXError(err, "设置长连接端点失败")
	}
	if !updateSuccess {
		return xerr.NewError("已有其他长连接端点")
	}
	return nil
}

func (cache *Cache) ConnGetEndpoint(ctx context.Context, uid lib.Int64S) (string, error) {
	val, err := rdb.GetRedisC().Get(ctx, cache._cometEndpoint(uid))
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", xerr.NewXError(err, "获取长连接端点失败")
	}
	return val, nil
}
func (cache *Cache) ConnDelEndpoint(ctx context.Context, uid lib.Int64S) error {
	err := rdb.GetRedisC().Del(ctx, cache._cometEndpoint(uid))
	if err != nil {
		return xerr.NewXError(err, "删除长连接端点地址失败")
	}
	return nil
}

func (cache *Cache) GetUserByLoginId(ctx context.Context, loginId string) (string, error) {
	if !strings.HasPrefix(loginId, keyLoginIdPrefix) {
		loginId = fmt.Sprintf(keyLoginIdPrefix_uuid, loginId)
	}

	val, err := rdb.GetRedisC().Get(ctx, loginId)
	if err != nil {
		return "", xerr.NewXError(err, "获取用户信息失败")
	}
	return val, nil
}

func (cache *Cache) GetRouteTag(ctx context.Context, id string) string {

	key := fmt.Sprintf(keyRouteTagPrefix_Id, id)
	val, err := rdb.GetRedisC().Get(ctx, key)
	if err != nil {
		val = "prod"
	}
	routeTag := val

	logx.DebugX("GetRouteTag")(id, key, val, "err", err, routeTag)
	return routeTag
}

func (cache *Cache) GetUserNameByUserId(ctx context.Context, userIdArr []string) (map[string]string, error) {
	if len(userIdArr) == 0 {
		return nil, nil
	}
	id2usernameMap, err := rdb.GetRedisC().HMGet(ctx, keyId2usernameMap, userIdArr...)
	if err != nil {
		return nil, xerr.NewXError(err, fmt.Sprintf("获取id到用户名的映射失败 %v", userIdArr))
	}
	return id2usernameMap, err
}
func (cache *Cache) HMSet(ctx context.Context, hName string, hashM map[string]string) error {

	keyValArr := []any{}
	for key, val := range hashM {
		keyValArr = append(keyValArr, key)
		keyValArr = append(keyValArr, val)
	}

	err := rdb.GetRedisC().HMSet(ctx, hName, keyValArr...)
	if err != nil {
		return xerr.NewXError(err, "更新"+hName)
	}
	return nil
}
func (cache *Cache) PutId2UserName(ctx context.Context, storeKeySuffix string, id2usernameMap map[string]string) error {

	id2usernameArr := []any{}
	for key, val := range id2usernameMap {
		id2usernameArr = append(id2usernameArr, key)
		id2usernameArr = append(id2usernameArr, val)
	}

	id2usernameMapStoreKey := keyId2usernameMap
	if storeKeySuffix != "" {
		id2usernameMapStoreKey = fmt.Sprintf(keyId2usernameMapPrefix_tmpStoreKey, storeKeySuffix)
	}
	err := rdb.GetRedisC().HMSet(ctx, id2usernameMapStoreKey, id2usernameArr...)
	if err != nil {
		return xerr.NewXError(err, "更新id到用户名的映射失败")
	}
	return nil
}
func (cache *Cache) PutUsername2Id(ctx context.Context, storeKeySuffix string, username2idMap map[string]string) error {

	username2idArr := []any{}
	for key, val := range username2idMap {
		username2idArr = append(username2idArr, key)
		username2idArr = append(username2idArr, val)
	}

	username2idMapStoreKey := keyUsername2IdMap
	if storeKeySuffix != "" {
		username2idMapStoreKey = fmt.Sprintf(keyUsername2IdMapPrefix_tmpStoreKey, storeKeySuffix)
	}
	err := rdb.GetRedisC().HMSet(ctx, username2idMapStoreKey, username2idArr...)
	if err != nil {
		return xerr.NewXError(err, "更新id到用户名的映射失败")
	}
	return nil
}

func (cache *Cache) RandomUserIdList(ctx context.Context, cnt int) ([]string, error) {
	totalCnt, err := rdb.GetRedisC().ZCount(ctx, keyLoginStatus, "-inf", "+inf")
	if err != nil {
		return nil, err
	}
	start := randx.RandomInt(int(totalCnt) - cnt)
	userIds, err := rdb.GetRedisC().ZRange(ctx, keyLoginStatus, int64(start), int64(start)+int64(cnt))
	return userIds, xerr.NewXError(err)
}

type ScanItem struct {
	Key string
	Val string
	Err error
}

func (cache *Cache) ScanAllId(ctx context.Context) chan *ScanItem {
	return cache.HScanAll(ctx, keyId2usernameMap)
}

func (cache *Cache) HScanAll(ctx context.Context, scanKey string) chan *ScanItem {
	ch := make(chan *ScanItem, 1024)

	go func() {
		defer close(ch)

		var cursor uint64 = 0
		runner.RunForever(ctx, fmt.Sprintf("scanAll.%v", scanKey), func(runCnt int) bool {
			if ctx.Err() != nil {
				ch <- &ScanItem{Err: xerr.NewXError(ctx.Err())}
				return false
			}
			nextCursor, data, err := rdb.GetRedisC().HScan(ctx, scanKey, cursor, "*", 1000)
			if err == redis.Nil {
				return false
			}
			if err != nil {
				item := &ScanItem{Err: xerr.NewXError(err)}
				ch <- item
				return false
			}
			for k, v := range data {
				item := &ScanItem{Key: k, Val: v}
				ch <- item
			}
			if nextCursor == 0 {
				return false
			}
			cursor = nextCursor
			return true
		})
	}()
	return ch
}

// userIdLoginTimeMap,
// userId: int64,
// loginTimeStamp: 秒数,从1960年开始到现在的秒数,
// 表示用户登录的时间截, -1 则是未登录,假设session 的时间是10分钟,那么登录时间截 < 当前时间减去10分钟 的用户都是未登录用户,
func (cache *Cache) UpdateLoginStatus(ctx context.Context, storeKeySuffix string, userIdLoginTimeMap map[string]float64) error {
	var arr []*redis.Z = make([]*redis.Z, 0, len(userIdLoginTimeMap))
	for userId, loginTimeStamp := range userIdLoginTimeMap {
		z := &redis.Z{
			Score:  float64(loginTimeStamp),
			Member: fmt.Sprintf("%v", userId),
		}
		arr = append(arr, z)
	}
	loginStatusStoreKey := keyLoginStatus
	if storeKeySuffix != "" {
		loginStatusStoreKey = fmt.Sprintf(keyLoginStatusPrefix_tmpStoreKey, storeKeySuffix)
	}

	_, err := rdb.GetRedisC().ZAdd(ctx, loginStatusStoreKey, arr...)
	if err != nil {
		return xerr.NewXError(err, "更新用户登录时间截失败")
	}
	err = rdb.GetRedisC().Expire(ctx, loginStatusStoreKey, time.Hour*12)
	if err != nil {
		return xerr.NewXError(err, "设置缓存键的过期时间失败")
	}
	return nil
}

func (cache *Cache) GetNotLoginUserIdList(ctx context.Context, count int64) ([]string, error) {
	// 获取分数小于50的元素（按分数升序排列）
	// 使用 "-inf" 表示负无穷，"(50" 表示小于50（不包含50）
	loginSession := appConfigLib.GetAppConfig().BConfig.Get("loginSession").(int)
	queryCond := &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("(%v", time.Now().Add(-time.Duration(loginSession)*time.Second).Unix()), // 登陆时间在 session 之前的用户
		Count: count,
	}
	idList, err := rdb.GetRedisC().ZRangeByScore(ctx, keyLoginStatus, queryCond)
	if err != nil {
		return nil, xerr.NewXError(err, fmt.Sprintf("查询未登录数据失败 %v", queryCond))
	}
	return idList, nil
}
func (cache *Cache) GetHadLoginUserCnt(ctx context.Context) (int64, error) {
	loginSession := appConfigLib.GetAppConfig().BConfig.Get("loginSession").(int)
	// 获取分数小于50的元素（按分数升序排列）
	// 使用 "-inf" 表示负无穷，"(50" 表示小于50（不包含50）
	min := fmt.Sprintf("(%v", time.Now().Add(-time.Duration(loginSession)*time.Second).Unix())
	max := "+inf"
	cnt, err := rdb.GetRedisC().ZCount(ctx, keyLoginStatus, min, max)
	if err != nil {
		return 0, xerr.NewXError(err, fmt.Sprintf("查询登录用户数失败 %v -> %v", min, max))
	}
	return cnt, nil
}
func (cache *Cache) GetNoLoginUserCnt(ctx context.Context) (int64, error) {
	loginSession := appConfigLib.GetAppConfig().BConfig.Get("loginSession").(int)
	// 获取分数小于50的元素（按分数升序排列）
	// 使用 "-inf" 表示负无穷，"(50" 表示小于50（不包含50）
	max := fmt.Sprintf("(%v", time.Now().Add(-time.Duration(loginSession)*time.Second).Unix())
	min := "-inf"
	cnt, err := rdb.GetRedisC().ZCount(ctx, keyLoginStatus, min, max)
	if err != nil {
		return 0, xerr.NewXError(err, fmt.Sprintf("查询未登录用户数失败 %v -> %v", min, max))
	}
	return cnt, nil
}

// 标记 登录状态初始化开始
// 标记 登录状态初始化完成
// 是否有正在进行的登录状态初始化进程
func (cache *Cache) EnterInitLoginStatus(ctx context.Context, force bool, taskFunc func(ctx context.Context, loginStatusSuffix string) error) (err error) {
	loginStatusOngoingFlagKey := "loginStatusOngoing"               // 标记是否有进程在进行"登录状态"的初始化
	tmpStoreKey := fmt.Sprintf("_tmp_%v", randx.RandomInt(1000000)) // 用于登录状态初始化的临时存储键
	// loginStatusNormalKey := "loginStatus" // 登录状态的正式存储键

	success, err := rdb.GetRedisC().SetNX(ctx, loginStatusOngoingFlagKey, tmpStoreKey, time.Second*100)
	if err != nil {
		return xerr.NewXError(err, "[登录状态] 尝试启动初始化失败")
	}

	if !success {
		if !force {
			return xerr.NewXError(err, "[登录状态] 已经有进程在进行初始化")
		}
		err := rdb.GetRedisC().Set(ctx, loginStatusOngoingFlagKey, tmpStoreKey, time.Second*100)
		if err != nil {
			return xerr.NewXError(err, "[登录状态] 尝试启动初始化失败")
		}
	}

	tmpKeyLoginStatus := fmt.Sprintf(keyLoginStatusPrefix_tmpStoreKey, tmpStoreKey)
	tmpKeyId2username := fmt.Sprintf(keyId2usernameMapPrefix_tmpStoreKey, tmpStoreKey)
	tmpKeyUsername2Id := fmt.Sprintf(keyUsername2IdMapPrefix_tmpStoreKey, tmpStoreKey)
	tmpKeyTableCnt := fmt.Sprintf(keyTableCntPrefix_tmpStoreKey, tmpStoreKey)

	logx.InfoX("zadd")(rdb.GetRedisC().ZAdd(ctx, tmpKeyLoginStatus, &redis.Z{Score: 0, Member: "hi"}))
	logx.InfoX("hmset")(rdb.GetRedisC().HMSet(ctx, tmpKeyId2username, "hi", "hi"))
	logx.InfoX("hmset")(rdb.GetRedisC().HMSet(ctx, tmpKeyUsername2Id, "hi", "hi"))
	logx.InfoX("hmset")(rdb.GetRedisC().HMSet(ctx, tmpKeyTableCnt, "hi", "hi"))

	errKey, err := xerr.AnyErr(map[string]error{
		tmpKeyLoginStatus: rdb.GetRedisC().Expire(ctx, tmpKeyLoginStatus, time.Second*100),
		tmpKeyId2username: rdb.GetRedisC().Expire(ctx, tmpKeyId2username, time.Second*100),
		tmpKeyUsername2Id: rdb.GetRedisC().Expire(ctx, tmpKeyUsername2Id, time.Second*100),
		tmpKeyTableCnt:    rdb.GetRedisC().Expire(ctx, tmpKeyTableCnt, time.Second*100),
	})
	logx.WhenErr(errKey)(err)

	defer func() {
		// 当 zset 为空时,这个键就不存在了,然后导致ticketRun 返回失败, 触发cancel
		// 因为后续会进行 rename 为 loginStatus, 所以 zRem 对象改为 loginStatus
		logx.InfoX("zrem")(rdb.GetRedisC().ZRem(ctx, keyLoginStatus, "hi"))
		logx.InfoX("hmdel")(rdb.GetRedisC().HMDel(ctx, keyId2usernameMap, "hi"))
		logx.InfoX("hmdel")(rdb.GetRedisC().HMDel(ctx, keyUsername2IdMap, "hi"))
		logx.InfoX("hmdel")(rdb.GetRedisC().HMDel(ctx, keyTableCnt, "hi"))
	}()

	ret, err2 := rdb.GetRedisC().Get(ctx, loginStatusOngoingFlagKey)
	logx.Infof("锁定初始化成功 %v %v, ret: %v, err2: %v", success, err, ret, err2)
	newCtx, cancelOngoingFlagKeyF := runner.RunWithTickerDuration(ctx, "cache.EnterInitLoginStatus", time.Second*3, func() bool {
		val, err := rdb.GetRedisC().Get(ctx, loginStatusOngoingFlagKey)
		if err != nil {
			logx.Errorf("%v", xerr.NewXError(err, "[登录状态] 获取正在进行的任务临时存储键失败 %v"))
			return true
		}
		if val != tmpStoreKey {
			// 已经被别的进程抢占, 当前就无需继续对期进行续期
			logx.Infof("%v 已被其他进程抢占,当前无需续期 redisVal: %v curSuffix: %v", loginStatusOngoingFlagKey, val, tmpStoreKey)
			return false
		}
		errKey, err := xerr.AnyErr(map[string]error{
			loginStatusOngoingFlagKey: rdb.GetRedisC().Expire(ctx, loginStatusOngoingFlagKey, time.Second*100),
			tmpKeyLoginStatus:         rdb.GetRedisC().Expire(ctx, tmpKeyLoginStatus, time.Second*100),
			tmpKeyId2username:         rdb.GetRedisC().Expire(ctx, tmpKeyId2username, time.Second*100),
			tmpKeyUsername2Id:         rdb.GetRedisC().Expire(ctx, tmpKeyUsername2Id, time.Second*100),
			tmpKeyTableCnt:            rdb.GetRedisC().Expire(ctx, tmpKeyTableCnt, time.Second*100),
		})
		if err != nil {
			logx.Errorf("%v %v", errKey, xerr.NewXError(err, "[登录状态] 对初始化过程续期失败"))
			return false
		}
		logx.Infof("%v 续期成功 redisVal: %v curSuffix: %v", loginStatusOngoingFlagKey, val, tmpStoreKey)
		return true
	})
	defer func() {
		logx.InfoX("[登录状态] 获取正在进行的任务临时存储键, 取消续期")
		cancelOngoingFlagKeyF()
		logx.InfoX("[登录状态] 获取正在进行的任务临时存储键, 取消续期 done")
	}()

	time.Sleep(13 * time.Second)
	// 任务执行在这里
	err = taskFunc(newCtx, tmpStoreKey)
	if err != nil {
		return xerr.NewXError(err, "[登录状态] 初始化执行")
	}
	logx.DebugX("数据初始化任务")(tmpStoreKey, err)

	errKey, err = xerr.AnyErr(map[string]error{
		"Rename." + tmpKeyLoginStatus:  rdb.GetRedisC().Rename(ctx, tmpKeyLoginStatus, keyLoginStatus),
		"Persist." + tmpKeyLoginStatus: rdb.GetRedisC().PersistX(ctx, keyLoginStatus),

		"Rename." + tmpKeyId2username:  rdb.GetRedisC().Rename(ctx, tmpKeyId2username, keyId2usernameMap),
		"Persist." + keyId2usernameMap: rdb.GetRedisC().PersistX(ctx, keyId2usernameMap),

		"Rename." + tmpKeyUsername2Id:  rdb.GetRedisC().Rename(ctx, tmpKeyUsername2Id, keyUsername2IdMap),
		"Persist." + keyUsername2IdMap: rdb.GetRedisC().PersistX(ctx, keyUsername2IdMap),

		"Rename." + tmpKeyTableCnt: rdb.GetRedisC().Rename(ctx, tmpKeyTableCnt, keyTableCnt),
		"Persist." + keyTableCnt:   rdb.GetRedisC().PersistX(ctx, keyTableCnt),
	})
	logx.WhenErr("[登录状态] 初始化失败 " + errKey)(err)
	if err != nil {
		return nil
	}

	logx.InfoX(fmt.Sprintf("[登录状态] 将临时存储键 %v 切换为正式键 %v done", tmpKeyLoginStatus, keyLoginStatus))
	err = rdb.GetRedisC().Del(ctx, loginStatusOngoingFlagKey)
	if err != nil {
		err = xerr.NewXError(err, "[登录状态] 删除进程正在进行的标记失败")
		return
	}
	return nil
}

func (cache *Cache) Reconciliation(ctx context.Context) (string, error) {

	retM, err := rdb.GetRedisC().HGetAll(ctx, "tableCnt")
	if err != nil {
		return "", err
	}
	totalCnt := retM["totalCnt"]
	delete(retM, "totalCnt")
	keys := runner.Keys(retM)
	slices.Sort(keys)
	s := ""
	var cntSum int64 = 0
	cntM, err := rdb.GetRedisC().MHCount(ctx, keys...)
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		cntVal := cntM[key]
		if cntVal.Val != nil {
			cntSum += cntVal.Val.(int64)
		}
		s += fmt.Sprintf("table: %v tableCnt: %v hashCnt: %v equal: %v \n", key, retM[key], cntVal.Val, retM[key] == fmt.Sprintf("%v", cntVal.Val))
	}
	return fmt.Sprintf("tableTotalCnt: %v hashTotalCnt: %v \n", totalCnt, cntSum) + s, nil
}

func (cache *Cache) CalUserCntBySum(ctx context.Context, storeKeySuffix string) (int64, error) {
	storeKey := keyTableCnt
	if storeKeySuffix != "" {
		storeKey = fmt.Sprintf(keyTableCntPrefix_tmpStoreKey, storeKeySuffix)
	}

	retM, err := rdb.GetRedisC().HGetAll(ctx, storeKey)
	if err != nil {
		return 0, err
	}
	var sum int64 = 0
	for k, v := range retM {
		if k == "totalCnt" {
			continue
		}
		val, _ := strconv.ParseInt(v, 10, 32)
		sum += val
	}
	return sum, nil
}

func (cache *Cache) UpdateUserTotalCnt(ctx context.Context, storeKeySuffix string, totalCnt int32) error {
	storeKey := keyTableCnt
	if storeKeySuffix != "" {
		storeKey = fmt.Sprintf(keyTableCntPrefix_tmpStoreKey, storeKeySuffix)
	}
	err := rdb.GetRedisC().HMSet(ctx, storeKey, "totalCnt", totalCnt)
	return err
}

func (cache *Cache) GetUserTotalCnt(ctx context.Context, storeKeySuffix string) (int64, error) {
	storeKey := keyTableCnt
	if storeKeySuffix != "" {
		storeKey = fmt.Sprintf(keyTableCntPrefix_tmpStoreKey, storeKeySuffix)
	}
	retM, err := rdb.GetRedisC().HMGet(ctx, storeKey, "totalCnt")
	return lib.ParseInt(retM["totalCnt"], err)
}
func (cache *Cache) UpdateUserTableCnt(ctx context.Context, storeKeySuffix string, tableName string, tableCnt int32) error {
	storeKey := keyTableCnt
	if storeKeySuffix != "" {
		storeKey = fmt.Sprintf(keyTableCntPrefix_tmpStoreKey, storeKeySuffix)
	}
	err := rdb.GetRedisC().HMSet(ctx, storeKey, tableName, tableCnt)
	return err
}

func (cache *Cache) GetUserTableCnt(ctx context.Context, storeKeySuffix string, tableName string) (map[string]string, error) {
	storeKey := keyTableCnt
	if storeKeySuffix != "" {
		storeKey = fmt.Sprintf(keyTableCntPrefix_tmpStoreKey, storeKeySuffix)
	}
	retM, err := rdb.GetRedisC().HMGet(ctx, storeKey, tableName)
	return retM, err
}
