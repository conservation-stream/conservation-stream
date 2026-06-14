import { type z } from 'zod';

import { createStore } from '../store';
import { config, services } from './config';

export const createEnvironment = async (env: Env) => {
  const variables = config.parse(env);
  const environment = await services({ ...env, ...variables });

  return {
    variables,
    ...environment
  };
};

export type Environment = Awaited<ReturnType<typeof createEnvironment>>;

const EnvironmentStore = createStore<Environment>('environment');
export const [withEnvironment, useEnvironment] = EnvironmentStore;
