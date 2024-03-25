package subscribe

import (
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"bgpvalidator/internal/rislive"
	"bgpvalidator/internal/validate"
)

var (
	SubscribeCmd = &cobra.Command{
		Use:   "subscribe",
		Short: "从ris live上获取bgp数据，并验证起源AS",
		Run:   subscribe,
	}
)

const RISLIVE_CONFIG = "rislive"
const validateUrl = "validate_url"

func subscribe(cmd *cobra.Command, args []string) {
	clientId := viper.Sub(RISLIVE_CONFIG).GetString("clientId") //用户端Id，其实没什么用
	duration := viper.Sub(RISLIVE_CONFIG).GetInt("duration")    //持续时间
	onlyIPv4 := viper.Sub(RISLIVE_CONFIG).GetBool("onlyIpv4")
	var filter *rislive.RisLiveClientMessageData
	err := viper.Sub(RISLIVE_CONFIG).Sub("filter").Unmarshal(&filter)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	handle, err := rislive.MakeRisLiveHandle(rislive.MakeRisLiveSubscribeUrl(clientId), filter, duration)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	scheme, host := viper.Sub(validateUrl).GetString("scheme"), viper.Sub(validateUrl).GetString("host")
	path := viper.Sub(validateUrl).GetString("path")
	validator := validate.MakeRoutinatorValidator(scheme, host, path)
	if validator != nil{
		handle.Process(onlyIPv4, validator)
	}
}
