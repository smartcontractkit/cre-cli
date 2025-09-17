import { cre } from "cre-sdk-typescript/src/sdk/cre";
import type { Runtime } from "cre-sdk-typescript/src/sdk/runtime/runtime";

type Config = {
  schedule: string;
};

function onCronTrigger(_config: Config, runtime: Runtime) {
  runtime.logger.log("Hello world! Workflow triggered.");
  cre.sendResponseValue(cre.utils.val.string("Hello world!"));
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
