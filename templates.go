package main

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/go-redis/redis"
)

var divider = "-------------------------------------\n"
var leaderboard = "{{if .Global}} **:earth_americas: | Global Fishy Leaderboards** {{else}} **:cityscape: | Guild Fishy Leaderboards for {{.GuildName}}** {{end}}\n" +
	"```pl\n📋 Rank | Name\n\n" +
	"{{range $i, $e := .Scores}}[{{$i}}]\t> #{{$e.Member}}\n\t\t\tTotal Points: {{$e.Score}}\n{{else}}:x: | No leaderboards\n{{end}}" +
	divider +
	"# Your {{if .Global}}Global{{else}}Guild{{end}} Placing Stats\n" +
	"😐 Rank: {{.Rank}}\tTotal Score: {{.Score}}\n```"

func LeaderboardTemp(scores []redis.Z, global bool, user string, guild string, guildName string) (string, error) {
	var doc bytes.Buffer
	var rank int64
	var score float64

	if global {
		rank, score = DBGetGlobalScoreRank(user)
	} else {
		rank, score = DBGetGuildScoreRank(user, guild)
	}
	data := LeaderboardData{scores, rank, score, guildName, global}
	tmpl, err := template.New("leaderboard").Parse(leaderboard)
	if err != nil {
		fmt.Println("Error parsing template", err.Error())
		return "", err
	}
	if err = tmpl.Execute(&doc, data); err != nil {
		fmt.Println("Error parsing template", err.Error())
		return "", err
	}

	return doc.String(), err
}
