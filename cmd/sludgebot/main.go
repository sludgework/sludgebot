package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/aprzybys/sludgebot/internal/services"
	"github.com/aprzybys/sludgebot/internal/services/webhooks"
	"github.com/aprzybys/sludgebot/internal/services/webhooks/msghook"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/mjwhitta/cli"
	"github.com/mjwhitta/log"
	"golang.org/x/sync/errgroup"
)

const back = "`"
const backs = "```"

type BotServer struct {
	Server *base.Server
	opts   Options
	kbc    *kbchat.API
}

type Options struct {
	MsghookOptions *msghook.MsghookOptions
	Options        *base.Options
}

// Exit status
const (
	Good = iota
	InvalidOption
	MissingOption
	InvalidArgument
	MissingArgument
	ExtraArgument
	Exception
)

// Flags
var flags struct {
	databaseURI    string
	httpPrefix     string
	webhookCommand string
}

func init() {
	cli.Align = true
	cli.Authors = []string{"Adam Przybyszewski <aprzybys@err.cx>"}
	cli.Banner = fmt.Sprintf("%s [OPTIONS] <arg>", os.Args[0])
	cli.BugEmail = "sludgebot-dev@err.cx"
	cli.ExitStatus(
		"Normally the exit status is 0. In the event of an error the",
		"exit status will be one of the below:\n\n",
		fmt.Sprintf("  %d: Invalid option\n", InvalidOption),
		fmt.Sprintf("  %d: Missing option\n", MissingOption),
		fmt.Sprintf("  %d: Invalid argument\n", InvalidArgument),
		fmt.Sprintf("  %d: Missing argument\n", MissingArgument),
		fmt.Sprintf("  %d: Extra argument\n", ExtraArgument),
		fmt.Sprintf("  %d: Exception", Exception),
	)
	cli.Info(
		"Sludgebot is the award-winning product resulting from years",
		"of research.",
	)
	cli.MaxWidth = 80
	cli.TabWidth = 4 // Defaults to 4
	cli.Title = "Sludgebot"

	// Parse cli flags
	cli.Flag(
		&flags.databaseURI,
		"d",
		"database",
		"",
		"Database URI in DSN format",
	)
	cli.Flag(
		&flags.httpPrefix,
		"msghook-prefix",
		"",
		"HTTP prefix for generated msghooks",
	)
	cli.Flag(
		&flags.webhookCommand,
		"msghook-cmd",
		"msghook",
		"Command run to access msghook functionality",
	)
	cli.Parse()
}

func main() {
	validate()

	// For base.NewOtions() - github.com/keybase/managed-bots/base:
	// ------------------------------------------------------------
	// type Options struct {
	// 	// Location of the keybase binary
	// 	KeybaseLocation string
	// 	// Home directory for keybase service
	// 	Home string
	// 	// Conversation name or ID to announce when the bot begins
	// 	Announcement string
	// 	// Conversation name or ID to report bot errors to
	// 	ErrReportConv string
	// 	// Database Source Name
	// 	DSN          string
	// 	MultiDSN     string
	// 	StathatEZKey string
	// 	// Allow the bot to read it's own messages (default: false)
	// 	ReadSelf bool
	// 	AWSOpts  *AWSOptions
	// }
	opts := &Options{
		MsghookOptions: &msghook.MsghookOptions{
			Command:    flags.webhookCommand,
			HTTPPrefix: flags.httpPrefix,
		},
		Options: base.NewOptions(),
	}
	opts.Options.DSN = flags.databaseURI

	// From "github.com/keybase/go-keybase-chat-bot/kbchat":
	// ------------------------------------------------------------
	// type RunOptions struct {
	//     KeybaseLocation string
	//     HomeDir         string
	//     Oneshot         *OneshotOptions
	//     StartService    bool
	//     // Have the bot send/receive typing notifications
	//     EnableTyping bool
	//     // Disable bot lite mode
	//     DisableBotLiteMode bool
	//     // Number of processes to spin up to connect to the keybase service
	//     NumPipes int
	// }
	var run_options = kbchat.RunOptions{
		KeybaseLocation: opts.Options.KeybaseLocation,
		HomeDir:         opts.Options.Home,
		NumPipes:        5,
	}

	// From github.com/keybase/managed-bots/base:
	// type Server struct {
	//     *DebugOutput
	//     sync.Mutex
	//     shutdownCh chan struct{}
	//     name         string
	//     announcement string
	//     awsOpts      *AWSOptions
	//     kbc          *kbchat.API
	//     botAdmins    []string
	//     multiDBDSN   string
	//     multi        *multi
	//     readSelf     bool
	//     runOptions kbchat.RunOptions
	// }
	var new_server = base.NewServer(
		"sludgebot",
		opts.Options.Announcement,
		opts.Options.AWSOpts,
		opts.Options.MultiDSN,
		opts.Options.ReadSelf,
		run_options,
	)

	var bs = &BotServer{
		Server: new_server,
		opts:   *opts,
	}

	log.Infof("Let's go!")
	if err := bs.Go(); err != nil {
		fmt.Printf("error running chat loop: %s\n", err)
		os.Exit(Exception)
	}

	os.Exit(Good)
}

func validate() {

	// Validate cli args
	if cli.NFlag() == 0 {
		cli.Usage(MissingArgument)
	} else if flags.databaseURI == "" {
		log.ErrfX(MissingOption, "Missing argument: %s", "database")
	} else if flags.httpPrefix == "" {
		log.ErrfX(MissingOption, "Missing argument: %s", "msghook-prefix")
	}
}

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	createExtended := fmt.Sprintf(`Create a new msghook for sending messages into the current conversation. You must supply a name as well to identify the msghook. To use a msghook URL, supply a %smsg%s URL parameter, or a JSON POST body with a field %smsg%s.

	Example:%s
		!msghook create alerts%s`, back, back, back, back, backs, backs)
	removeExtended := fmt.Sprintf(`Remove a msghook from the current conversation. You must supply the name of the msghook.

	Example:%s
		!msghook remove alerts%s`, backs, backs)

	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "msghook create",
			Description: "Create a new msghook for sending into the current conversation",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title: `*!msghook create* <name>
Create a msghook`,
				DesktopBody: createExtended,
				MobileBody:  createExtended,
			},
		},
		{
			Name:        "msghook list",
			Description: "List active msghooks in the current conversation",
		},
		{
			Name:        "msghook remove",
			Description: "Remove a msghook from the current conversation",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title: `*!msghook remove* <name>
Remove a msghook`,
				DesktopBody: removeExtended,
				MobileBody:  removeExtended,
			},
		},
		base.GetFeedbackCommandAdvertisement(s.kbc.GetUsername()),
	}
	return kbchat.Advertisement{
		Alias: "msghooks",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ:      "public",
				Commands: cmds,
			},
		},
	}
}

func (s *BotServer) Go() (err error) {
	if s.kbc, err = s.Server.Start(s.opts.Options.ErrReportConv); err != nil {
		return err
	}

	sqlDb, err := sql.Open("mysql", s.opts.Options.DSN)
	if err != nil {
		log.Errf("Failed to connect to MySQL: %s", err)
		return err
	}
	defer sqlDb.Close()

	log.Infof("Testing SQL Connection")
	err = sqlDb.Ping()
	if err != nil {
		log.Errf("Could not ping sqlDb: %s", err.Error())
		return err
	}

	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.Options.ErrReportConv)
	stats, err := base.NewStatsRegistry(debugConfig, s.opts.Options.StathatEZKey)
	if err != nil {
		s.Server.Debug("unable to create stats: %v", err)
		return err
	}
	stats = stats.SetPrefix(s.Server.Name())

	var webhookHandler *webhooks.WebhookHandler = &webhooks.WebhookHandler{
		DB:         webhooks.NewDB(sqlDb),
		HttpPrefix: s.opts.MsghookOptions.HTTPPrefix,
	}

	httpSrv := services.NewHTTPSrv(stats, debugConfig, webhookHandler.DB)

	h := services.NewHandler(stats, s.kbc, debugConfig, httpSrv, webhookHandler)
	eg := &errgroup.Group{}
	s.Server.GoWithRecover(eg, func() error { return s.Server.Listen(h) })
	s.Server.GoWithRecover(eg, httpSrv.Listen)
	s.Server.GoWithRecover(eg, func() error { return s.Server.HandleSignals(httpSrv, stats) })
	s.Server.GoWithRecover(eg, func() error { return s.Server.AnnounceAndAdvertise(s.makeAdvertisement(), "I live.") })
	if err := eg.Wait(); err != nil {
		s.Server.Debug("wait error: %s", err)
		return err
	}
	return nil
}
