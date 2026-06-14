import { motion } from 'motion/react';
import { useMemo } from 'react';
import type { FC } from 'react';
import { ButterflyIcon } from './ButterflyIcon';
import { flightPath, wingFlap } from './butterfly-keyframes';

interface AnimatedButterflyProps {
  fill?: string;
  duration?: number;
  reverse?: boolean;
  className?: string;
}

export const AnimatedButterfly: FC<AnimatedButterflyProps> = ({ fill = '#d24d34', duration = 8, reverse = false, className }) => {
  const path = useMemo(
    () => ({
      x: reverse ? [...flightPath.x].reverse() : flightPath.x,
      y: reverse ? [...flightPath.y].reverse() : flightPath.y
    }),
    [reverse]
  );

  return (
    <motion.div
      className={className}
      animate={{
        x: path.x,
        y: path.y,
        ...wingFlap,
        transition: {
          duration,
          ease: 'circInOut',
          repeat: Infinity,
          repeatType: 'mirror'
        }
      }}
      style={{ width: 10, height: 10 }}
    >
      <ButterflyIcon fill={fill} className="size-full" />
    </motion.div>
  );
};
