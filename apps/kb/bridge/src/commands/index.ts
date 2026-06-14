import { publishCommand } from './publish.ts';
import { updateCommand } from './update.ts';
import type { CommandContext } from './publish.ts';
import type { ValidCommands } from '../types/outline.ts';

const commands = {
  '/publish': publishCommand,
  '/update': updateCommand
} as const;

export const executeCommand = async (command: ValidCommands, ctx: CommandContext) => {
  const handler = commands[command];
  if (!handler) {
    throw new Error(`Unknown command: ${command}`);
  }
  return await handler(ctx);
};
