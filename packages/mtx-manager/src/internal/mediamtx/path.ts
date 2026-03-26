import { z } from 'zod';

// Main path config schema
export const PathConfig = z
  .object({
    source: z.string(),
    sourceFingerprint: z.string(),
    sourceOnDemand: z.boolean(),
    sourceOnDemandStartTimeout: z.string(),
    sourceOnDemandCloseAfter: z.string(),
    maxReaders: z.number(),
    srtReadPassphrase: z.string(),
    fallback: z.string(),
    useAbsoluteTimestamp: z.boolean(),
    alwaysAvailable: z.boolean(),
    alwaysAvailableTracks: z.unknown().array(),
    alwaysAvailableFile: z.string(),

    // Recording
    record: z.boolean(),
    recordPath: z.string(),
    recordFormat: z.string(),
    recordPartDuration: z.string(),
    recordMaxPartSize: z.string(),
    recordSegmentDuration: z.string(),
    recordDeleteAfter: z.string(),

    // Publishing
    overridePublisher: z.boolean(),
    srtPublishPassphrase: z.string(),
    rtspDemuxMpegts: z.boolean(),

    // RTSP
    rtspTransport: z.string(),
    rtspAnyPort: z.boolean(),
    rtspRangeType: z.string(),
    rtspRangeStart: z.string(),
    rtspUDPSourcePortRange: z.number().array(),

    // RTP
    rtpSDP: z.string(),

    // WebRTC / WHEP
    whepBearerToken: z.string(),
    whepSTUNGatherTimeout: z.string(),
    whepHandshakeTimeout: z.string(),
    whepTrackGatherTimeout: z.string(),

    // Redirect
    sourceRedirect: z.string(),

    // Raspberry Pi Camera
    rpiCameraCamID: z.number(),
    rpiCameraSecondary: z.boolean(),
    rpiCameraWidth: z.number(),
    rpiCameraHeight: z.number(),
    rpiCameraHFlip: z.boolean(),
    rpiCameraVFlip: z.boolean(),
    rpiCameraBrightness: z.number(),
    rpiCameraContrast: z.number(),
    rpiCameraSaturation: z.number(),
    rpiCameraSharpness: z.number(),
    rpiCameraExposure: z.string(),
    rpiCameraAWB: z.string(),
    rpiCameraAWBGains: z.tuple([z.number(), z.number()]),
    rpiCameraDenoise: z.string(),
    rpiCameraShutter: z.number(),
    rpiCameraMetering: z.string(),
    rpiCameraGain: z.number(),
    rpiCameraEV: z.number(),
    rpiCameraROI: z.string(),
    rpiCameraHDR: z.boolean(),
    rpiCameraTuningFile: z.string(),
    rpiCameraMode: z.string(),
    rpiCameraFPS: z.number(),
    rpiCameraAfMode: z.string(),
    rpiCameraAfRange: z.string(),
    rpiCameraAfSpeed: z.string(),
    rpiCameraLensPosition: z.number(),
    rpiCameraAfWindow: z.string(),
    rpiCameraFlickerPeriod: z.number(),
    rpiCameraTextOverlayEnable: z.boolean(),
    rpiCameraTextOverlay: z.string(),
    rpiCameraCodec: z.string(),
    rpiCameraIDRPeriod: z.number(),
    rpiCameraBitrate: z.number(),
    rpiCameraHardwareH264Profile: z.string(),
    rpiCameraHardwareH264Level: z.string(),
    rpiCameraSoftwareH264Profile: z.string(),
    rpiCameraSoftwareH264Level: z.string(),
    rpiCameraMJPEGQuality: z.number(),

    // Run hooks
    runOnInit: z.string(),
    runOnInitRestart: z.boolean(),
    runOnDemand: z.string(),
    runOnDemandRestart: z.boolean(),
    runOnDemandStartTimeout: z.string(),
    runOnDemandCloseAfter: z.string(),
    runOnUnDemand: z.string(),
    runOnReady: z.string(),
    runOnReadyRestart: z.boolean(),
    runOnNotReady: z.string(),
    runOnRead: z.string(),
    runOnReadRestart: z.boolean(),
    runOnUnread: z.string(),
    runOnRecordSegmentCreate: z.string(),
    runOnRecordSegmentComplete: z.string()
  })
  .partial();

// Type export
export type PathConfig = z.infer<typeof PathConfig>;
