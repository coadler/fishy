package main

import (
	"bytes"
	"fmt"
	"text/template"
)

var divider = "-------------------------------------\n"
var leaderboard = "{{if .Global}} **:earth_americas: | Global Fishy Leaderboards** {{else}} **:cityscape: | Guild Fishy Leaderboards for {{.GuildName}}** {{end}}\n" +
	"```pl\n📋 Rank | Name\n\n" +
	"{{range $i, $e := .Scores}}[{{inc $i}}]\t> # {{$e.GetUsername}}\n\t\t\tTotal Points: {{$e.Score}}\n{{else}}No leaderboards\n{{end}}" +
	divider +
	"# Your {{if .Global}}Global{{else}}Guild{{end}} Placing Stats\n" +
	"😐 Rank: {{inc64 .Rank}}\tTotal Score: {{.Score}}\n```"

func LeaderboardTemp(scores []LeaderboardUser, global bool, user string, guild string, guildName string) (string, error) {
	var doc bytes.Buffer
	var rank int64
	var score float64
	var funcMap = template.FuncMap{
		"inc64": func(i int64) int64 {
			return i + 1
		},
		"inc": func(i int) int {
			return i + 1
		},
	}

	if global {
		rank, score = DBGetGlobalScoreRank(user)
	} else {
		rank, score = DBGetGuildScoreRank(user, guild)
	}
	data := LeaderboardData{scores, rank, score, guildName, global}
	tmpl, err := template.New("leaderboard").Funcs(funcMap).Parse(leaderboard)
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

func (u LeaderboardUser) GetUsername() string {
	return DBGetTrackedUser(u.Member.(string))
}
