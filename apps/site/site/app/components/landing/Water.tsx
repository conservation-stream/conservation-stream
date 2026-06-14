import reflectionImage from '@/assets/reflection-first-frame-alpha.webp';
import reflectionHevc from '@/assets/reflection-hevc.mov';
import reflection from '@/assets/reflection.webm';
import { cn } from '@/lib/utils';
import Bowser from 'bowser';
import { useState, type ComponentProps, type FC } from 'react';

const supportsHEVCAlpha = (): boolean => {
  if (typeof window === 'undefined') return false;

  const parser = Bowser.getParser(window.navigator.userAgent);
  const browserName = parser.getBrowserName(true);
  const osName = parser.getOSName(true);
  const hasMediaCapabilities = !!window.navigator.mediaCapabilities?.decodingInfo;
  const isSafari = browserName === 'safari';
  const isIOS = osName === 'ios';

  return hasMediaCapabilities && (isSafari || isIOS);
};

export const Reflection: FC<ComponentProps<'video'>> = ({ className }) => {
  const [loaded, setLoaded] = useState(false);
  const videoSrc = supportsHEVCAlpha() ? reflectionHevc : reflection;

  return (
    <div className={cn('w-full h-full relative opacity-50 aspect-ratio-[2406/684]', className)}>
      {!loaded && <img src={reflectionImage} alt="Reflection" className={cn('absolute inset-0 w-full h-full object-cover')} />}
      <video
        src={videoSrc}
        autoPlay
        muted
        loop
        playsInline
        preload="metadata"
        className={cn('absolute inset-0 w-full h-full object-cover')}
        onCanPlay={async () => {
          await new Promise(resolve => setTimeout(resolve, 100));
          setLoaded(true);
        }}
      />
    </div>
  );
};
