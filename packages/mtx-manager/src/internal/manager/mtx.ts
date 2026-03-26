import type { GlobalConfig } from '../mediamtx/global';
import type { PathConfig } from '../mediamtx/path';

export type MediaMTXConfig = {
  pathDefaults?: PathConfig;
  paths?: Record<string, PathConfig>;
} & GlobalConfig;
