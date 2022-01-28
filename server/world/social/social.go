package social

import (
	"context"
	"database/sql"
	"gonet/actor"
	"gonet/base"
	"gonet/db"
	"gonet/server/model"
	"gonet/server/world"
	"gonet/server/world/player"
)

const(
	Temp = iota			// 临时
	Friend	= iota			// 好友
	Consort = iota			// 配偶
	Master	= iota			// 师傅
	Prentice = iota		// 徒弟
	Enemy = iota			// 仇人
	Mute = iota			// 屏蔽(黑名单)
	Count = iota

	sqlTable = "tbl_social"
)

//enSocialError
const (
	SocialError_None 		= iota
	SocialError_Unknown	= iota			// 未知
	SocialError_Self		= iota				// 不能和自身成为社会关系
	SocialError_MaxCount	= iota			// 此社会关系人数达到最大上限
	SocialError_NotFound	= iota			// 目标玩家不存在
	SocialError_Existed	= iota			// 目标玩家已是此在社会关系列表中
	SocialError_Unallowed	= iota	    // 该操作不允许
	SocialError_DbError	= iota	       	// 数据库操作错误
)

//分布式考虑直接数据库
type (
	SOCIALITEMMAP map[int64] *model.SocialItem

	SocialMgr struct{
		actor.Actor

		m_db *sql.DB
		m_Log *base.CLog
	}

	ISocialMgr interface {
		actor.IActor

		makeLink(PlayerId, TargetId int64, Type int8) int//加好友
		destoryLink(PlayerId, TargetId int64, Type int8) int//删除好友
		addFriendValue(PlayerId, TargetId int64, Value int) int//增加好友度
		loadSocialDB(PlayerId int64, Type int8) SOCIALITEMMAP
		loadSocialById(PlayerId, TargetId int64, Type int8) *model.SocialItem
		isFriendType(Type int8) bool
		isBestFriendType(Type int8) bool
		hasMakeLink(oldType, newType int8) bool
	}
)

var(
	g_pMgr actor.IActor
)

func MGR() actor.IActor{
	if g_pMgr == nil{
		g_pMgr = &SocialMgr{}
		if world.CONF.Redis.OpenFlag{
			g_pMgr = &SocialMgrR{}
		}else{
			g_pMgr = &SocialMgr{}
		}
	}
	return g_pMgr
}

func getSocialTypeMaxCount(Type int8) int{
	if Type >= Count ||  Type < 0{
		return 0
	}

	SocialTypeMaxCount	:= [Count]int{
		50, 50, 1, 1, 5, 20, 100,
	}

	return  SocialTypeMaxCount[Type]
}

func (this *SocialMgr) Init() {
	this.Actor.Init()
	this.m_db = world.SERVER.GetDB()
	this.m_Log = world.SERVER.GetLog()
	actor.MGR.RegisterActor(this)
	this.Actor.Start()
}

func (this *SocialMgr) isFriendType(Type int8) bool{
	if Type != Temp && Type != Mute && Type != Enemy{
		return	true
	}
	return  false
}

func (this *SocialMgr) isBestFriendType(Type int8) bool{
	if Type != Friend && Type != Temp && Type != Mute && Type != Enemy{
		return true
	}
	return	false
}

func (this *SocialMgr) hasMakeLink(oldType, newType int8) bool{
	if oldType == newType{
		return false
	}

	if oldType != Temp && newType == Temp{
		return false
	}

	if (this.isBestFriendType(oldType) || this.isBestFriendType(newType)){
		return false
	}

	if (oldType == Friend || oldType == Mute) && newType == Enemy{
		return false
	}

	return true
}

func loadSocialDB(row db.IRow, s *model.SocialItem){
	s.PlayerId = row.Int64("player_id")
	s.TargetId = row.Int64("target_id")
	s.Type = int8(row.Int("type"))
	s.FriendValue = row.Int("friend_value")
}

func (this *SocialMgr) loadSocialDB(PlayerId int64, Type int8) SOCIALITEMMAP{
	SocialMap := make(SOCIALITEMMAP)
	Item := &model.SocialItem{}
	rows, err := this.m_db.Query(db.LoadSql(Item, db.WithWhere(model.SocialItem{PlayerId:PlayerId, Type:Type})))
	rs := db.Query(rows, err)
	if rs.Next(){
		loadSocialDB(rs.Row(), Item)
		SocialMap[Item.TargetId] = Item
	}
	return SocialMap
}

func (this *SocialMgr) loadSocialById(PlayerId, TargetId int64, Type int8) *model.SocialItem{
	Item := &model.SocialItem{PlayerId:PlayerId, TargetId:TargetId, Type:Type}
	rows, err := this.m_db.Query(db.LoadSql(Item))
	rs := db.Query(rows, err)
	if rs.Next(){
		loadSocialDB(rs.Row(), Item)
		return Item
	}
	return nil
}

func (this *SocialMgr) 	makeLink(PlayerId, TargetId int64, Type int8) int{
	if Type >= Count{
		return SocialError_Unknown
	}

	if this.isBestFriendType(Type){
		return SocialError_Unallowed
	}

	if PlayerId == TargetId{
		return SocialError_Self
	}

	SocialMap := this.loadSocialDB(PlayerId, Type)
	Item, exist := SocialMap[TargetId]
	if exist{
		firstPlayerId := int64(0)
		nCount := len(SocialMap)

		//找一个空的
		for _, v := range SocialMap{
			if v.TargetId != TargetId{
				if firstPlayerId == 0{
					firstPlayerId = v.TargetId
					break
				}
			}
		}

		// type is full, send error
		if nCount > getSocialTypeMaxCount(Type){
			if Type != Enemy && Type != Temp{
				return SocialError_MaxCount
			}else{
				// 当仇人或临时好友添加满时，删除一个仇人或临时好友
				if firstPlayerId != 0 && SocialError_None != this.destoryLink( PlayerId, firstPlayerId, Type){
					return SocialError_DbError
				}
			}
		}

		if Item != nil{
			if Item.Type == Type{
				return SocialError_Existed
			}else{
				if !this.hasMakeLink(Item.Type, Type){
					return SocialError_Unallowed
				}
			}

			Item.PlayerId = PlayerId
			Item.TargetId = TargetId
			Item.Type = Type
			Item.FriendValue = 0
			this.m_db.Exec(db.UpdateSql(Item))
			this.m_Log.Printf("更新社会关系playerId=%d,destPlayerId=%d,newType=%d", PlayerId, TargetId, Item.Type)
			return SocialError_None
		}
	}else{
		Item := &model.SocialItem{}
		Item.PlayerId = PlayerId
		Item.TargetId = TargetId
		Item.Type = Type
		Item.FriendValue = 0

		SocialMap[TargetId] = Item
		this.m_db.Exec(db.InsertSql(Item))
		this.m_Log.Printf("新增社会关系playerId=%d,destPlayerId=%d,type=%d", PlayerId, TargetId, Item.Type)
		return SocialError_None
	}

	return SocialError_DbError
}

func (this *SocialMgr) destoryLink(PlayerId, TargetId int64, Type int8) int{
	if PlayerId == TargetId{
		return  SocialError_Self
	}

	Item := &model.SocialItem{}
	Item.PlayerId = PlayerId
	Item.TargetId = TargetId
	Item.Type = Type
	Item.FriendValue = 0


	if Item.Type == Friend || Item.Type == Mute || Item.Type == Temp || Item.Type == Enemy{
		this.m_db.Exec(db.DeleteSql(Item))
		return SocialError_None
	}

	return SocialError_Unallowed
}

func (this *SocialMgr) addFriendValue(PlayerId, TargetId int64, Value int) int{
	Item1 := this.loadSocialById(PlayerId, TargetId, Friend)
	Item2 := this.loadSocialById(TargetId, PlayerId, Friend)

	if Item1 == nil || Item2 == nil{
		return 0
	}

	if !this.isFriendType(Item1.Type) || !this.isFriendType(Item2.Type){
		return 0
	}

	Item1.FriendValue +=Value
	Item2.FriendValue = Item1.FriendValue
	this.m_db.Exec(db.UpdateSql(Item1))
	this.m_db.Exec(db.UpdateSql(Item2))
	return Value
}

func (this *SocialMgr) C_W_MakeLinkRequest(ctx context.Context, PlayerId, TargetId int64, Type int8) {
	pPlayer := player.SIMPLEMGR.GetPlayerDataById(PlayerId)
	pTarget	:= player.SIMPLEMGR.GetPlayerDataById(TargetId)
	if pPlayer == nil || pTarget == nil{
		this.m_Log.Printf("查询玩家id[%d][%d]数据为空", PlayerId, TargetId)
		return
	}
}