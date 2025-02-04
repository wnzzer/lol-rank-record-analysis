package handlers

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"lol-record-analysis/lcu/client"
	"lol-record-analysis/util/init_log"
	"math"
	"net/http"
	"strings"
	"sync"
)

type RecentData struct {
	KDA                           float64                    `json:"kda"`
	Kills                         float64                    `json:"kills"`
	Deaths                        float64                    `json:"deaths"`
	Assists                       float64                    `json:"assists"`
	Wins                          int                        `json:"wins"`
	Losses                        int                        `json:"losses"`
	FlexWins                      int                        `json:"flexWins"`
	FlexLosses                    int                        `json:"flexLosses"`
	GroupRate                     int                        `json:"groupRate"`
	AverageGold                   int                        `json:"averageGold"`
	GoldRate                      int                        `json:"goldRate"`
	AverageDamageDealtToChampions int                        `json:"averageDamageDealtToChampions"`
	DamageDealtToChampionsRate    int                        `json:"damageDealtToChampionsRate"`
	OneGamePlayersMap             map[string][]OneGamePlayer `json:"oneGamePlayers"` // 遇到用户的 puuid
}
type OneGamePlayer struct {
	Index          int    `json:"index"` //用于标记第几页,第几个
	GameCreatedAt  string `json:"gameCreatedAt"`
	GameId         int    `json:"gameId"`
	Puuid          string `json:"puuid"`
	GameName       string `json:"gameName"`
	TagLine        string `json:"tagLine"`
	ChampionId     int    `json:"championId"`
	ChampionBase64 string `json:"championBase64"`
	Win            bool   `json:"win"`
	Kills          int    `json:"kills"`
	Deaths         int    `json:"deaths"`
	Assists        int    `json:"assists"`
	IsMyTeam       bool   `json:"isMyTeam"`
}

type RankTag struct {
	Good    bool   `json:"good"`
	TagName string `json:"tagName"`
	TagDesc string `json:"tagDesc"`
}

type UserTag struct {
	RecentData RecentData `json:"recentData"`
	Tag        []RankTag  `json:"tag"`
}

func GetTag(c *gin.Context) {
	puuid := c.DefaultQuery("puuid", "")
	name := c.DefaultQuery("name", "")
	userTag, err := GetTagCore(puuid, name, false)
	if err != nil {
		init_log.AppLog.Error("GetTagCore() failed,%v", err)
		c.JSON(http.StatusOK, gin.H{
			"error": err.Error(),
		})
	} else {
		c.JSON(http.StatusOK, userTag)
	}

}

func GetTagCore(puuid string, name string, boolOneGamePlayers bool) (*UserTag, error) {

	if name != "" {
		summoner, _ := client.GetSummonerByName(name)
		puuid = summoner.Puuid
	}

	if puuid == "" {
		summoner, _ := client.GetCurSummoner()
		puuid = summoner.Puuid
	}

	if puuid == "" {
		return nil, errors.New("puuid or name is empty")
	} else {
		matchHistory, _ := client.GetMatchHistoryByPuuid(puuid, 0, 19)

		var wg sync.WaitGroup
		for i, games := range matchHistory.Games.Games {

			wg.Add(1)
			go func(i int, gameId int) {
				defer wg.Done()

				// 获取游戏详情
				gameDetail, err := client.GetGameDetail(gameId)
				if err != nil {
					// 错误处理：你可以在此记录错误日志或采取其他措施
					return
				}

				matchHistory.Games.Games[i].GameDetail = gameDetail
			}(i, games.GameId)
		}
		wg.Wait()

		var tags []RankTag
		//判断是否是连胜
		streakTag := isStreakTag(&matchHistory)
		if streakTag.TagName != "" {
			tags = append(tags, streakTag)
		}
		//判断是否连败
		losingTag := isLosingTag(&matchHistory)
		if losingTag.TagName != "" {
			tags = append(tags, losingTag)
		}
		//判断是否是娱乐玩家
		casualTag := isCasualTag(&matchHistory)
		if casualTag.TagName != "" {
			tags = append(tags, casualTag)
		}
		//判断是否是特殊玩家
		specialPlayerTag := isSpecialPlayerTag(&matchHistory)
		if len(specialPlayerTag) > 0 {
			tags = append(tags, specialPlayerTag...)
		}

		//获取该玩家局内的所有玩家
		var oneGamePlayerMap map[string][]OneGamePlayer
		if boolOneGamePlayers {
			oneGamePlayerMap = getOneGamePlayers(&matchHistory)
		}

		//计算 kda,胜率,参团率,伤害转换率
		kills, death, assists := countKda(&matchHistory)
		kda := (kills + assists) / death
		kda = math.Trunc(kda*10) / 10
		kills = math.Trunc(kills*10) / 10
		death = math.Trunc(death*10) / 10
		assists = math.Trunc(assists*10) / 10

		wins, losses, flexWins, flexLosses := countWinAndLoss(&matchHistory)
		groupRate, averageGold, goldRate, averageDamageDealtToChampions, DamageDealtToChampionsRate := countGoldAndGroupAndDamageDealtToChampions(&matchHistory)
		userTag := UserTag{
			RecentData: RecentData{
				KDA:                           kda,
				Kills:                         kills,
				Deaths:                        death,
				Assists:                       assists,
				Wins:                          wins,
				Losses:                        losses,
				FlexWins:                      flexWins,
				FlexLosses:                    flexLosses,
				GroupRate:                     groupRate,
				AverageGold:                   averageGold,
				GoldRate:                      goldRate,
				AverageDamageDealtToChampions: averageDamageDealtToChampions,
				DamageDealtToChampionsRate:    DamageDealtToChampionsRate,
				OneGamePlayersMap:             oneGamePlayerMap,
			},
			Tag: tags,
		}
		return &userTag, nil
	}
}

func getOneGamePlayers(matchHistory *client.MatchHistory) map[string][]OneGamePlayer {
	oneGamePlayerMap := make(map[string][]OneGamePlayer)
	for index, games := range matchHistory.Games.Games {
		myTeamId := games.Participants[0].TeamId
		for i := 0; i < len(games.GameDetail.ParticipantIdentities); i++ {
			oneGamePlayerMap[games.GameDetail.ParticipantIdentities[i].Player.Puuid] = append(oneGamePlayerMap[games.GameDetail.ParticipantIdentities[i].Player.Puuid], OneGamePlayer{
				Index:          index,
				GameId:         games.GameId,
				GameCreatedAt:  games.GameCreationDate,
				IsMyTeam:       myTeamId == games.GameDetail.Participants[i].TeamId,
				GameName:       games.GameDetail.ParticipantIdentities[i].Player.SummonerName,
				TagLine:        games.GameDetail.ParticipantIdentities[i].Player.TagLine,
				ChampionId:     games.GameDetail.Participants[i].ChampionId,
				ChampionBase64: client.GetChampionBase64ById(games.GameDetail.Participants[i].ChampionId),
				Kills:          games.GameDetail.Participants[i].Stats.Kills,
				Deaths:         games.GameDetail.Participants[i].Stats.Deaths,
				Assists:        games.GameDetail.Participants[i].Stats.Assists,
				Win:            games.GameDetail.Participants[i].Stats.Win,
			})
		}
	}
	return oneGamePlayerMap
}

func countGoldAndGroupAndDamageDealtToChampions(matchHistory *client.MatchHistory) (int, int, int, int, int) {
	count := 1
	myGold := 0
	allGold := 1
	myKA := 0
	allK := 1
	myDamageDealtToChampions := 0
	allDamageDealtToChampions := 1
	for _, games := range matchHistory.Games.Games {
		if games.QueueId != client.QueueSolo5x5 && games.QueueId != client.QueueFlex {
			continue
		}
		for _, participant0 := range games.Participants {
			myGold += participant0.Stats.GoldEarned
			myKA += participant0.Stats.Kills
			myKA += participant0.Stats.Assists
			myDamageDealtToChampions += participant0.Stats.TotalDamageDealtToChampions
			for _, participant := range games.GameDetail.Participants {
				if participant0.TeamId == participant.TeamId {
					allGold += participant.Stats.GoldEarned
					allK += participant.Stats.Kills
					allDamageDealtToChampions += participant.Stats.TotalDamageDealtToChampions
				}
			}
		}
		count++
	}
	groupRate := math.Trunc(float64(myKA) / float64(allK) * 100)
	averageGold := math.Trunc(float64(myGold) / float64(count))
	goldRate := math.Trunc(float64(myGold) / float64(allGold) * 100)
	averageDamageDealtToChampions := math.Trunc(float64(myDamageDealtToChampions) / float64(count))
	damageDealtToChampionsRate := math.Trunc(float64(myDamageDealtToChampions) / float64(allDamageDealtToChampions) * 100)
	return int(groupRate), int(averageGold), int(goldRate), int(averageDamageDealtToChampions), int(damageDealtToChampionsRate)
}
func countWinAndLoss(matchHistory *client.MatchHistory) (int, int, int, int) {
	wins := 0
	losses := 0
	flexWins := 0
	flexLosses := 0
	for _, games := range matchHistory.Games.Games {

		if games.QueueId == client.QueueSolo5x5 {
			if games.Participants[0].Stats.Win == true {
				wins++
			} else {
				losses++
			}
		}
		if games.QueueId == client.QueueFlex {
			if games.Participants[0].Stats.Win == true {
				flexWins++
			} else {
				flexLosses++

			}
		}

	}
	return wins, losses, flexWins, flexLosses

}
func countKda(matchHistory *client.MatchHistory) (float64, float64, float64) {
	count := 1
	kills := 0
	deaths := 1
	assists := 0
	for _, games := range matchHistory.Games.Games {
		if games.QueueId != client.QueueSolo5x5 && games.QueueId != client.QueueFlex {
			continue
		}
		count++
		kills += games.Participants[0].Stats.Kills
		deaths += games.Participants[0].Stats.Deaths
		assists += games.Participants[0].Stats.Assists
	}
	return float64(float32(kills) / float32(count)), float64(float32(deaths) / float32(count)), float64(float32(assists) / float32(count))
}

func isStreakTag(matchHistory *client.MatchHistory) RankTag {
	des := "最近胜率较高的大腿玩家哦"

	i := 0
	for _, games := range matchHistory.Games.Games {
		//不是排位不算
		if games.QueueId != client.QueueSolo5x5 && games.QueueId != client.QueueFlex {
			continue
		}
		if games.Participants[0].Stats.Win == false {
			break
		}
		i++
	}
	if i >= 3 {
		tag := fmt.Sprintf("%s连胜", numberToChinese(i))
		return RankTag{Good: true, TagName: tag, TagDesc: des}
	} else {
		return RankTag{}
	}

}
func isLosingTag(matchHistory *client.MatchHistory) RankTag {
	desc := "最近连败的玩家哦"

	i := 0
	for _, games := range matchHistory.Games.Games {
		if games.QueueId != client.QueueSolo5x5 && games.QueueId != client.QueueFlex {
			continue
		}
		if games.Participants[0].Stats.Win == true {
			break
		}
		i++
	}
	if i >= 3 {
		tag := fmt.Sprintf("%s连败", numberToChinese(i))
		return RankTag{Good: false, TagName: tag, TagDesc: desc}
	} else {
		return RankTag{}
	}

}
func isCasualTag(matchHistory *client.MatchHistory) RankTag {
	desc := "排位比例较少的玩家哦,请宽容一点"
	i := 0
	for _, games := range matchHistory.Games.Games {
		if games.QueueId != client.QueueSolo5x5 && games.QueueId != client.QueueFlex {
			i++
		}
	}
	if i > 10 {
		tag := "娱乐"
		return RankTag{Good: false, TagName: tag, TagDesc: desc}
	} else {
		return RankTag{}
	}
}
func isSpecialPlayerTag(matchHistory *client.MatchHistory) []RankTag {
	var tags []RankTag
	var BadSpecialChampion = map[int]string{
		901: "小火龙",
		141: "凯隐",
		10:  "天使",
	}
	desc := "该玩家使用上述英雄比例较高(由于英雄特殊定位,风评相对糟糕的英雄玩家)"
	//糟糕英雄标签选取

	var badSpecialChampionSelectMap = map[string]int{}
	for _, games := range matchHistory.Games.Games {
		if games.QueueId != client.QueueSolo5x5 && games.QueueId != client.QueueFlex {
			continue
		}
		championName, _ := BadSpecialChampion[games.Participants[0].ChampionId]
		if championName != "" {
			if _, ok := badSpecialChampionSelectMap[championName]; ok {
				badSpecialChampionSelectMap[championName]++
			} else {
				badSpecialChampionSelectMap[championName] = 1
			}
		}
	}
	for tagName, useCount := range badSpecialChampionSelectMap {
		if useCount >= 5 {
			tags = append(tags, RankTag{Good: false, TagName: tagName, TagDesc: desc})
		}
	}
	return tags
}

// 将数字转换为中文
func numberToChinese(num int) string {
	var chineseDigits = []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	var chineseUnits = []string{"", "十", "百", "千", "万", "亿"}
	if num == 0 {
		return chineseDigits[0]
	}

	var result []string
	unitPos := 0
	zeroFlag := false

	for num > 0 {
		// 获取当前数字的个位数
		digit := num % 10
		if digit == 0 {
			if !zeroFlag && len(result) > 0 {
				result = append(result, chineseDigits[0])
			}
			zeroFlag = true
		} else {
			result = append(result, chineseDigits[digit]+chineseUnits[unitPos])
			zeroFlag = false
		}
		num /= 10
		unitPos++
	}

	// 处理"一十" -> "十"
	if len(result) > 1 && result[len(result)-1] == chineseUnits[1] {
		result = result[:len(result)-1]
	}

	// 反转结果并拼接
	for i := len(result)/2 - 1; i >= 0; i-- {
		opp := len(result) - 1 - i
		result[i], result[opp] = result[opp], result[i]
	}

	return strings.Join(result, "")
}
