import { z } from 'zod';

// Reusable sub-schemas
export const Permission = z.object({
  action: z.string(),
  path: z.string()
});

export const AuthInternalUser = z.object({
  user: z.string(),
  pass: z.string(),
  ips: z.string().array(),
  permissions: Permission.array()
});

export const IceServer = z.object({
  url: z.string(),
  username: z.string(),
  password: z.string(),
  clientOnly: z.boolean()
});

// Main global config schema
export const GlobalConfig = z
  .object({
    logLevel: z.literal(['debug', 'info', 'warn', 'error']),
    logDestinations: z.string().array(),
    logStructured: z.boolean(),
    logFile: z.string(),
    sysLogPrefix: z.string(),
    dumpPackets: z.boolean(),

    readTimeout: z.string(),
    writeTimeout: z.string(),
    writeQueueSize: z.number(),
    udpMaxPayloadSize: z.number(),
    udpReadBufferSize: z.number(),

    // Run hooks
    runOnConnect: z.string(),
    runOnConnectRestart: z.boolean(),
    runOnDisconnect: z.string(),

    // Authentication
    authMethod: z.literal(['internal', 'http', 'jwt']),
    authInternalUsers: AuthInternalUser.array(),
    authHTTPAddress: z.string(),
    authHTTPFingerprint: z.string(),
    authHTTPExclude: Permission.array(),
    authJWTJWKS: z.string(),
    authJWTJWKSFingerprint: z.string(),
    authJWTClaimKey: z.string(),
    authJWTExclude: Permission.array(),
    authJWTInHTTPQuery: z.boolean(),
    authJWTIssuer: z.string(),
    authJWTAudience: z.string(),

    // API
    api: z.boolean(),
    apiAddress: z.string(),
    apiEncryption: z.boolean(),
    apiServerKey: z.string(),
    apiServerCert: z.string(),
    apiAllowOrigins: z.string().array(),
    apiTrustedProxies: z.string().array(),

    // Metrics
    metrics: z.boolean(),
    metricsAddress: z.string(),
    metricsEncryption: z.boolean(),
    metricsServerKey: z.string(),
    metricsServerCert: z.string(),
    metricsAllowOrigins: z.string().array(),
    metricsTrustedProxies: z.string().array(),

    // Pprof
    pprof: z.boolean(),
    pprofAddress: z.string(),
    pprofEncryption: z.boolean(),
    pprofServerKey: z.string(),
    pprofServerCert: z.string(),
    pprofAllowOrigins: z.string().array(),
    pprofTrustedProxies: z.string().array(),

    // Playback
    playback: z.boolean(),
    playbackAddress: z.string(),
    playbackEncryption: z.boolean(),
    playbackServerKey: z.string(),
    playbackServerCert: z.string(),
    playbackAllowOrigins: z.string().array(),
    playbackTrustedProxies: z.string().array(),

    // RTSP
    rtsp: z.boolean(),
    rtspTransports: z.string().array(),
    rtspEncryption: z.literal(['no', 'strict', 'optional']),
    rtspAddress: z.string(),
    rtspsAddress: z.string(),
    rtpAddress: z.string(),
    rtcpAddress: z.string(),
    multicastIPRange: z.string(),
    multicastRTPPort: z.number(),
    multicastRTCPPort: z.number(),
    srtpAddress: z.string(),
    srtcpAddress: z.string(),
    multicastSRTPPort: z.number(),
    multicastSRTCPPort: z.number(),
    rtspServerKey: z.string(),
    rtspServerCert: z.string(),
    rtspAuthMethods: z.string().array(),

    // RTMP
    rtmp: z.boolean(),
    rtmpAddress: z.string(),
    rtmpEncryption: z.string(),
    rtmpsAddress: z.string(),
    rtmpServerKey: z.string(),
    rtmpServerCert: z.string(),

    // HLS
    hls: z.boolean(),
    hlsAddress: z.string(),
    hlsEncryption: z.boolean(),
    hlsServerKey: z.string(),
    hlsServerCert: z.string(),
    hlsAllowOrigins: z.string().array(),
    hlsTrustedProxies: z.string().array(),
    hlsAlwaysRemux: z.boolean(),
    hlsVariant: z.string(),
    hlsSegmentCount: z.number(),
    hlsSegmentDuration: z.string(),
    hlsPartDuration: z.string(),
    hlsSegmentMaxSize: z.string(),
    hlsDirectory: z.string(),
    hlsMuxerCloseAfter: z.string(),

    // WebRTC
    webrtc: z.boolean(),
    webrtcAddress: z.string(),
    webrtcEncryption: z.boolean(),
    webrtcServerKey: z.string(),
    webrtcServerCert: z.string(),
    webrtcAllowOrigins: z.string().array(),
    webrtcTrustedProxies: z.string().array(),
    webrtcLocalUDPAddress: z.string(),
    webrtcLocalTCPAddress: z.string(),
    webrtcIPsFromInterfaces: z.boolean(),
    webrtcIPsFromInterfacesList: z.string().array(),
    webrtcAdditionalHosts: z.string().array(),
    webrtcICEServers2: IceServer.array(),
    webrtcHandshakeTimeout: z.string(),
    webrtcTrackGatherTimeout: z.string(),
    webrtcSTUNGatherTimeout: z.string(),

    // SRT
    srt: z.boolean(),
    srtAddress: z.string()
  })
  .partial();

// Type exports
export type Permission = z.infer<typeof Permission>;
export type AuthInternalUser = z.infer<typeof AuthInternalUser>;
export type IceServer = z.infer<typeof IceServer>;
export type GlobalConfig = z.infer<typeof GlobalConfig>;
