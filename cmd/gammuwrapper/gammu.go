package main

import (
	"context"
	"os/exec"
	"strconv"

	"github.com/guigui42/gammuwrapper/conf"
	"github.com/rs/zerolog/log"
)

func sendSMS(ctx context.Context, sms SMS) ([]byte, error) {

	cmd := exec.CommandContext(ctx, "gammu", `-c`, conf.Conf.GammuConf, `--sendsms`, `TEXT`, sms.PhoneNumber, `-text`, sms.Message, "-autolen", strconv.Itoa(len(sms.Message)))

	log.Info().Msg(cmd.Dir)
	log.Info().Msg("Sending SMS")
	log.Info().Msgf("Command: %v", cmd)

	// cmd output :
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msgf("Command Error: %v", err)
		log.Error().Msgf("Command Output: %v", string(output))

		return output, err
	}
	log.Info().Msgf("Command Output: %v", string(output))
	return output, nil
}
