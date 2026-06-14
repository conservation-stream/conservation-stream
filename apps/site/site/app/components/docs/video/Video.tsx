import { Pause, Play } from 'lucide-react';
import { useEffect, useRef, useState, type ComponentProps, type FC } from 'react';

const getThumbnail = (url: URL) => {
  const id = url.pathname.split('/').pop()?.replace('.m3u8', '');
  return id ? `https://image.mux.com/${id}/thumbnail.jpg?time=0` : undefined;
};

export const Video: FC<ComponentProps<'video'>> = ({ src }) => {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const observerRef = useRef<IntersectionObserver | null>(null);
  const [hasFinished, setHasFinished] = useState(false);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);

  useEffect(() => {
    setHasFinished(false);
    setIsPlaying(false);
    setCurrentTime(0);
    setDuration(0);

    observerRef.current?.disconnect();
    observerRef.current = null;

    const node = videoRef.current;
    if (!node) return;

    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (!entry.isIntersecting) return;
          void node.play().catch(() => {
            setIsPlaying(false);
          });
        });
      },
      { threshold: 0.5 }
    );

    observer.observe(node);
    observerRef.current = observer;

    return () => {
      observer.disconnect();
      observerRef.current = null;
    };
  }, [src]);

  if (!src) return null;

  const url = new URL(src, window.location.origin);
  const aspectRatio = url.searchParams.get('aspect-ratio');
  const thumbnail = getThumbnail(url);
  const progress = duration > 0 ? currentTime / duration : 0;

  return (
    <div className="w-full max-w-3xl mx-auto">
      <div className="w-full bg-gray-200 h-auto rounded-lg overflow-clip mx-auto relative">
        <video
          ref={videoRef}
          muted
          playsInline
          preload="metadata"
          onPlay={() => setIsPlaying(true)}
          onPause={() => setIsPlaying(false)}
          onTimeUpdate={event => {
            setCurrentTime(event.currentTarget.currentTime);
          }}
          onLoadedMetadata={event => {
            setDuration(event.currentTarget.duration || 0);
          }}
          onEnded={() => {
            setHasFinished(true);
            setIsPlaying(false);
          }}
          className="w-3xl z-10"
          style={{
            background: thumbnail ? `url(${thumbnail}) no-repeat center center` : undefined,
            backgroundSize: 'cover',
            backgroundPosition: 'center',
            aspectRatio: aspectRatio ? `1/${aspectRatio}` : undefined
          }}
          src={src}
        />
        <div
          className="absolute inset-x-0 bottom-0 h-1 bg-[#F9712E] z-0"
          style={{
            width: `${progress * 100}%`
          }}
        />
        {hasFinished ? (
          <div className="absolute inset-0">
            <button
              onClick={() => {
                const node = videoRef.current;
                if (!node) return;
                if (node.paused) {
                  void node.play();
                  return;
                }
                node.pause();
              }}
              className="absolute left-3 bottom-3 w-8 h-8 rounded-sm backdrop-blur-xs bg-black/10 hover:bg-black/20 transition-all duration-200 flex items-center justify-center"
            >
              {isPlaying ? <Pause strokeWidth={0} fill="currentColor" className="text-white w-5 h-5" /> : <Play strokeWidth={0} fill="currentColor" className="text-white w-5 h-5" />}
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
};
