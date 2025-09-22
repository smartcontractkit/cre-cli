import { cre } from "@chainlink/cre-sdk/cre";
import { Value } from "@chainlink/cre-sdk/utils/values/value";
import type { Runtime } from "@chainlink/cre-sdk/runtime/runtime";

type Config = {
  schedule: string;
};

function onCronTrigger(_config: Config, runtime: Runtime) {
  runtime.logger.log("Hello world! Workflow triggered.");
  cre.sendResponseValue(Value.from("Hello world!"));
};

function initWorkflow(config: Config) {
  const cron = new cre.capabilities.CronCapability();

  return [
    cre.handler(
      cron.trigger({
        schedule: config.schedule,
      }),
      onCronTrigger
    ),
  ];
};

export async function main() {
  const runner = await cre.newRunner();

  await runner.run(initWorkflow);
}

main();
