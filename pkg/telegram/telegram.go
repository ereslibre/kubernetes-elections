package telegram

import (
	"context"
	"fmt"
	"log"
	"os"

	"crypto/sha1"

	"github.com/go-telegram-bot-api/telegram-bot-api"

	electionsv1beta1 "github.com/ereslibre/kubernetes-elections/pkg/apis/elections/v1beta1"
	"github.com/ereslibre/kubernetes-elections/pkg/utils"
	machineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type stage int

const (
	welcome        stage = 0
	electionChoice stage = 1
	castingBallot  stage = 2
)

var (
	users map[int64]*UserStage = map[int64]*UserStage{}
)

type UserStage struct {
	stage            stage
	message          *tgbotapi.MessageConfig
	currentElections *electionsv1beta1.ElectionList
	currentElection  *electionsv1beta1.Election
	currentBallot    *electionsv1beta1.Ballot
	currentQuestion  string
	currentOptions   []string
}

func fetchWelcome(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "¡Welcome to SUSE Containers Day!, let's vote")
	welcomeButtons := [][]tgbotapi.KeyboardButton{
		[]tgbotapi.KeyboardButton{
			tgbotapi.NewKeyboardButton("/vote"),
		},
	}
	welcomeButtonOptions := tgbotapi.NewReplyKeyboard(welcomeButtons...)
	msg.ReplyMarkup = welcomeButtonOptions
	return msg
}

func fetchElection(elections *electionsv1beta1.ElectionList, electionName string) *electionsv1beta1.Election {
	for _, election := range elections.Items {
		if election.ObjectMeta.Name == electionName {
			return &election
		}
	}
	return nil
}

func fetchElections(bot *tgbotapi.BotAPI, c client.Client, update tgbotapi.Update) {
	userStage := getStage(update)

	var elections *electionsv1beta1.ElectionList

	if userStage.currentElections == nil {
		elections = &electionsv1beta1.ElectionList{}
		c.List(context.TODO(), client.InNamespace("default"), elections)
	} else {
		elections = userStage.currentElections
	}

	var msg tgbotapi.MessageConfig

	if len(elections.Items) == 0 {
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, "It seems that there are no elections at the moment")
		msg.ReplyMarkup = tgbotapi.NewHideKeyboard(false)
	} else {
		electionNames := []string{}
		for _, election := range elections.Items {
			electionNames = append(electionNames, election.ObjectMeta.Name)
		}

		if utils.ContainsString(electionNames, update.Message.Text) {
			userStage.currentElection = fetchElection(elections, update.Message.Text)
			setStage(update, castingBallot)
			castBallot(bot, c, update)
		} else {
			electionButtons := []tgbotapi.KeyboardButton{}
			for _, election := range elections.Items {
				electionButtons = append(electionButtons, tgbotapi.NewKeyboardButton(election.ObjectMeta.Name))
			}
			electionButtonsKeyboard := [][]tgbotapi.KeyboardButton{
				electionButtons,
			}
			electionButtonOptions := tgbotapi.NewReplyKeyboard(electionButtonsKeyboard...)

			if !update.Message.IsCommand() {
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "That election doesn't seem to exist")
			} else {
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Choose an election to vote")
			}
			msg.ReplyMarkup = electionButtonOptions
		}
	}

	bot.Send(msg)
}

func currentAnsweredQuestions(update tgbotapi.Update) []string {
	userStage := getStage(update)

	answers := []string{}
	for answer, _ := range userStage.currentBallot.Spec.Answers {
		answers = append(answers, answer)
	}

	return answers
}

func castBallot(bot *tgbotapi.BotAPI, c client.Client, update tgbotapi.Update) {
	userStage := getStage(update)

	voteId := fmt.Sprintf("vote-%s-%s", userStage.currentElection.ObjectMeta.Name, fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("%d", update.Message.Chat.ID)))))

	ballot := &electionsv1beta1.Ballot{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: voteId, Namespace: userStage.currentElection.ObjectMeta.Namespace}, ballot)
	if !machineryerrors.IsNotFound(err) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Oops, it looks like you already voted in this election")
		msg.ReplyMarkup = tgbotapi.NewHideKeyboard(false)
		bot.Send(msg)

		setStage(update, welcome)

		return
	}

	if len(currentAnsweredQuestions(update)) != len(userStage.currentElection.Spec.Options) {
		if utils.ContainsString(userStage.currentOptions, update.Message.Text) {
			userStage.currentBallot.Spec.Answers[userStage.currentQuestion] = update.Message.Text
		}

		for question, options := range userStage.currentElection.Spec.Options {
			if utils.ContainsString(currentAnsweredQuestions(update), question) {
				continue
			}
			userStage.currentQuestion = question
			userStage.currentOptions = options
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, question)
			ballotButtons := []tgbotapi.KeyboardButton{}
			for _, option := range options {
				ballotButtons = append(ballotButtons, tgbotapi.NewKeyboardButton(option))
			}
			ballotButtonsKeyboard := [][]tgbotapi.KeyboardButton{
				ballotButtons,
			}
			ballotButtonOptions := tgbotapi.NewReplyKeyboard(ballotButtonsKeyboard...)
			msg.ReplyMarkup = ballotButtonOptions
			bot.Send(msg)
			break
		}
	}

	if len(currentAnsweredQuestions(update)) == len(userStage.currentElection.Spec.Options) {
		userStage.currentBallot.Spec.Election = userStage.currentElection.ObjectMeta.Name
		userStage.currentBallot.ObjectMeta.Name = voteId
		userStage.currentBallot.ObjectMeta.Namespace = userStage.currentElection.ObjectMeta.Namespace
		c.Create(context.TODO(), userStage.currentBallot)

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Thanks for voting, vote registered!")
		msg.ReplyMarkup = tgbotapi.NewHideKeyboard(false)
		bot.Send(msg)

		setStage(update, welcome)
	}
}

func dontUnderstand(bot *tgbotapi.BotAPI, c client.Client, update tgbotapi.Update) {
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "I don't understand you, sorry"))
	if userStage, ok := users[update.Message.Chat.ID]; ok && userStage.message != nil {
		bot.Send(userStage.message)
	}
}

func getStage(update tgbotapi.Update) *UserStage {
	return users[update.Message.Chat.ID]
}

func setStage(update tgbotapi.Update, stage stage) {
	userStage, ok := users[update.Message.Chat.ID]
	if !ok {
		userStage = &UserStage{
			stage: stage,
		}
	} else {
		userStage.stage = stage
	}
	switch stage {
	case welcome, electionChoice:
		userStage.currentElection = &electionsv1beta1.Election{}
		userStage.currentBallot = &electionsv1beta1.Ballot{
			Spec: electionsv1beta1.BallotSpec{
				Answers: map[string]string{},
			},
		}
	case castingBallot:
		userStage.currentBallot = &electionsv1beta1.Ballot{
			Spec: electionsv1beta1.BallotSpec{
				Answers: map[string]string{},
			},
		}
	}
	users[update.Message.Chat.ID] = userStage
}

func Bot(m manager.Manager) {
	c := m.GetClient()

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start", "cancel":
				setStage(update, welcome)
			case "vote":
				setStage(update, electionChoice)
			default:
				dontUnderstand(bot, c, update)
				continue
			}
		}

		userStage := getStage(update)
		if userStage == nil {
			setStage(update, welcome)
			userStage = getStage(update)
		}

		switch userStage.stage {
		case welcome:
			bot.Send(fetchWelcome(update))
			setStage(update, electionChoice)
		case electionChoice:
			fetchElections(bot, c, update)
		case castingBallot:
			castBallot(bot, c, update)
		}
	}
}
