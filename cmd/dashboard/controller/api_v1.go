package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/naiba/nezha/model"
	"github.com/naiba/nezha/pkg/mygin"
	"github.com/naiba/nezha/service/singleton"
)

type apiV1 struct {
	r gin.IRouter
}

func (v *apiV1) serve() {
	r := v.r.Group("")
	// API
	r.Use(mygin.Authorize(mygin.AuthorizeOption{
		Member:   true,
		IsPage:   false,
		AllowAPI: true,
		Msg:      "访问此接口需要认证",
		Btn:      "点此登录",
		Redirect: "/login",
	}))
	r.GET("/server/list", v.serverList)
	r.GET("/server/details", v.serverDetails)
	r.POST("/server/add", v.addOrEditServer)

}

func (ma *apiV1) addOrEditServer(c *gin.Context) {
	var sf serverForm
	var s model.Server
	var isEdit bool
	err := c.ShouldBindJSON(&sf)
	if err == nil {
		s.Name = sf.Name
		s.Secret = sf.Secret
		s.DisplayIndex = sf.DisplayIndex
		s.ID = sf.ID
		s.Tag = sf.Tag
		s.Note = sf.Note
		s.HideForGuest = sf.HideForGuest == "on"

		var ss model.Server
		e := singleton.DB.Model(&model.Server{}).Where("name = ?", s.Name).First(&ss).Error
		if e == nil {
			s.ID = ss.ID
		}

		// if s.Secret == "" {
		// 	s.Secret, _ = utils.GenerateRandomString(18)
		// }

		if s.ID == 0 {

			if err == nil {
				err = singleton.DB.Create(&s).Error
			}

		} else {
			isEdit = true
			err = singleton.DB.Save(&s).Error
		}
	}
	if err != nil {
		c.JSON(http.StatusOK, model.Response{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("请求错误：%s", err),
		})
		return
	}
	if isEdit {
		singleton.ServerLock.Lock()
		s.CopyFromRunningServer(singleton.ServerList[s.ID])
		// 如果修改了 Secret
		if s.Secret != singleton.ServerList[s.ID].Secret {
			// 删除旧 Secret-ID 绑定关系
			singleton.SecretToID[s.Secret] = s.ID
			// 设置新的 Secret-ID 绑定关系
			delete(singleton.SecretToID, singleton.ServerList[s.ID].Secret)
		}
		// 如果修改了Tag
		oldTag := singleton.ServerList[s.ID].Tag
		newTag := s.Tag
		if newTag != oldTag {
			index := -1
			for i := 0; i < len(singleton.ServerTagToIDList[oldTag]); i++ {
				if singleton.ServerTagToIDList[oldTag][i] == s.ID {
					index = i
					break
				}
			}
			if index > -1 {
				// 删除旧 Tag-ID 绑定关系
				singleton.ServerTagToIDList[oldTag] = append(singleton.ServerTagToIDList[oldTag][:index], singleton.ServerTagToIDList[oldTag][index+1:]...)
				if len(singleton.ServerTagToIDList[oldTag]) == 0 {
					delete(singleton.ServerTagToIDList, oldTag)
				}
			}
			// 设置新的 Tag-ID 绑定关系
			singleton.ServerTagToIDList[newTag] = append(singleton.ServerTagToIDList[newTag], s.ID)
		}
		singleton.ServerList[s.ID] = &s
		singleton.ServerLock.Unlock()
	} else {
		s.Host = &model.Host{}
		s.State = &model.HostState{}
		singleton.ServerLock.Lock()
		singleton.SecretToID[s.Secret] = s.ID
		singleton.ServerList[s.ID] = &s
		singleton.ServerTagToIDList[s.Tag] = append(singleton.ServerTagToIDList[s.Tag], s.ID)
		singleton.ServerLock.Unlock()
	}
	singleton.ReSortServer()
	c.JSON(http.StatusOK, model.Response{
		Code: http.StatusOK,
	})
}

// serverList 获取服务器列表 不传入Query参数则获取全部
// header: Authorization: Token
// query: tag (服务器分组)
func (v *apiV1) serverList(c *gin.Context) {
	tag := c.Query("tag")
	if tag != "" {
		c.JSON(200, singleton.ServerAPI.GetListByTag(tag))
		return
	}
	c.JSON(200, singleton.ServerAPI.GetAllList())
}

// serverDetails 获取服务器信息 不传入Query参数则获取全部
// header: Authorization: Token
// query: id (服务器ID，逗号分隔，优先级高于tag查询)
// query: tag (服务器分组)
func (v *apiV1) serverDetails(c *gin.Context) {
	var idList []uint64
	idListStr := strings.Split(c.Query("id"), ",")
	if c.Query("id") != "" {
		idList = make([]uint64, len(idListStr))
		for i, v := range idListStr {
			id, _ := strconv.ParseUint(v, 10, 64)
			idList[i] = id
		}
	}
	tag := c.Query("tag")
	if tag != "" {
		c.JSON(200, singleton.ServerAPI.GetStatusByTag(tag))
		return
	}
	if len(idList) != 0 {
		c.JSON(200, singleton.ServerAPI.GetStatusByIDList(idList))
		return
	}
	c.JSON(200, singleton.ServerAPI.GetAllStatus())
}
