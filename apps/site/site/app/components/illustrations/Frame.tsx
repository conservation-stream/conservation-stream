import { motion } from 'motion/react';
import type { FC, PropsWithChildren } from 'react';

interface FrameProps {
  position: {
    /** 0 = left edge, 1 = right edge */
    x: number;
    /** 0 = top edge, 1 = bottom edge */
    y: number;
  };
  size: {
    percentage?: { width: number; height: number };
    absolute?: { width: number; height: number };
  };
  corner: number;
  shadow?: boolean;
}

const CORNERS = [
  { position: 'top-left', rotate: '0deg' },
  { position: 'top-right', rotate: '90deg' },
  { position: 'bottom-right', rotate: '180deg' },
  { position: 'bottom-left', rotate: '270deg' }
] as const;

const CornerBracket: FC<{ corner: number; rotate: string; style: React.CSSProperties }> = ({ corner, rotate, style }) => (
  <svg className="absolute stroke-1 lg:stroke-2" style={{ ...style, rotate }} width={corner * 3} height={corner * 3} viewBox="0 0 5 5" fill="none">
    <path d="M4 1H1V4" stroke="white" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

const FrameShadow: FC = () => (
  <svg className="absolute -bottom-[20%] left-1/2 w-[130%] -translate-x-1/2 opacity-15" viewBox="0 0 135 17" fill="none">
    <g filter="url(#frame-shadow)" style={{ mixBlendMode: 'multiply' }}>
      <ellipse cx="67.3" cy="8.30005" rx="62" ry="3" fill="black" />
    </g>
    <defs>
      <filter id="frame-shadow" x="4.86374e-05" y="4.86374e-05" width="134.6" height="16.6" filterUnits="userSpaceOnUse" colorInterpolationFilters="sRGB">
        <feFlood floodOpacity="0" result="BackgroundImageFix" />
        <feBlend mode="normal" in="SourceGraphic" in2="BackgroundImageFix" result="shape" />
        <feGaussianBlur stdDeviation="2.65" result="effect1_foregroundBlur" />
      </filter>
    </defs>
  </svg>
);

function cornerStyle(pos: (typeof CORNERS)[number]['position'], offset: number): React.CSSProperties {
  const styles: Record<string, React.CSSProperties> = {
    'top-left': { top: -offset, left: -offset },
    'top-right': { top: -offset, right: -offset },
    'bottom-right': { bottom: -offset, right: -offset },
    'bottom-left': { bottom: -offset, left: -offset }
  };
  return styles[pos];
}

export const Frame: FC<PropsWithChildren<FrameProps>> = ({ position, size, children, corner, shadow }) => (
  <motion.div
    whileHover={{
      backgroundColor: 'rgba(255, 255, 255, 0.1)',
      scale: 1.02
    }}
    className="absolute rounded-[5px] border-2 border-white/10 pointer-events-auto"
    style={{
      left: `${position.x * 100}%`,
      top: `${position.y * 100}%`,
      width: size.percentage ? `${size.percentage.width * 100}%` : `${size.absolute?.width}rem`,
      height: size.percentage ? `${size.percentage.height * 100}%` : `${size.absolute?.height}rem`
    }}
  >
    {children}
    {CORNERS.map(({ position: pos, rotate }) => (
      <CornerBracket key={pos} corner={corner} rotate={rotate} style={cornerStyle(pos, corner)} />
    ))}
    {shadow && <FrameShadow />}
  </motion.div>
);
