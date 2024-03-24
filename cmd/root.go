package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"bgpvalidator/cmd/subscribe"
)

var (
	// Used for flags.
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "bgpValidator",
		Short: "基于RIS-LIVE的实时BGP UPDATE流的BGP数据收集器",
	}
)

// 命令行执行
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件 (默认为 [$HOME|$CURRENT_WORKDIR]/config.json)")
	rootCmd.AddCommand(subscribe.SubscribeCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile) //使用config字符串指向的路径上的配置文件
	} else {
		home, err := os.UserHomeDir() // 查找home目录
		cobra.CheckErr(err)           //如果home目录不存在就报错
		viper.AddConfigPath(".")      //添加当前目录到查找配置文件的路径列表中
		viper.AddConfigPath(home)     //添加home目录到查找配置文件的路径列表中
		viper.SetConfigType("json")   //配置文件的类型
		viper.SetConfigName("config") //配置文件的名称
	}

	if err := viper.ReadInConfig(); err == nil {
		slog.Info("Using config file", "fileName", viper.ConfigFileUsed())
	} else {
		slog.Error(err.Error())
	}
}
