import backgroundImage from "@/assets/background.webp";
import cloudsImage from "@/assets/clouds.webp";
import { AnimatedButterfly } from "@/components/illustrations/AnimatedButterfly";
import { BirdIcon } from "@/components/illustrations/BirdIcon";
import { Frame } from "@/components/illustrations/Frame";
import { Reflection } from "@/components/landing/Water";
import { Button } from "@/components/ui/button";
import { Outlet } from "react-router";

export default function Docs() {
  return (
    <div className="overflow-clip text-foreground max-w-400 mx-auto min-[100rem]:mt-8 min-[100rem]:rounded-t-3xl">
      <div className="relative bg-docs-gradient">
        <div className="absolute inset-0 z-10">
          <Frame
            position={{ x: 0.85, y: 0.2 }}
            size={{ absolute: { width: 6, height: 4 } }}
            corner={3.5}
          >
            <BirdIcon className="mx-8 my-4 size-6" />
          </Frame>
        </div>

        <div
          style={{ backgroundImage: `url(${cloudsImage})` }}
          className="z-0 w-full animate-drift-nne bg-amber-50 pb-15 flex flex-col"
        >
          <div className="pt-8 pb-8 px-10 flex justify-center items-center z-20">
            <Button>Get Started</Button>
          </div>
          <div className="relative mb-[min(10vw,160px)] flex flex-col gap-4 items-center px-6 z-10">
            <div className="pointer-events-none absolute left-1/2 top-1/2 h-44 w-[min(92vw,700px)] -translate-x-1/2 -translate-y-1/2 rounded-full bg-white/60 blur-3xl" />
            <h1 className="relative z-10 text-3xl sm:text-5xl font-semibold text-balance w-full max-w-xl text-center">
              Next generation conservation
            </h1>
            <p className="relative z-10 text-balance max-w-2xl text-center">
              We're a collaborative network of organizations, communities, and
              individuals who believe 24/7 live cameras and streaming are
              powerful conservation tools.
            </p>
          </div>
        </div>

        <div className="relative -mt-[min(10vw,160px)] z-10 pointer-events-none select-none">
          <img src={backgroundImage} alt="" className="relative z-20 w-full" />
          <Reflection className="absolute z-10 top-[26%] left-1/2 h-[25%] w-[70%] -translate-x-1/2" />

          <div className="absolute inset-0 z-20">
            <Frame
              shadow
              position={{ x: 0.07, y: 0.28 }}
              size={{ percentage: { width: 0.14, height: 0.3 } }}
              corner={3.5}
            />
            <Frame
              shadow
              position={{ x: 0.77, y: 0.18 }}
              size={{ percentage: { width: 0.12, height: 0.25 } }}
              corner={3.5}
            />
            <Frame
              shadow
              position={{ x: 0.43, y: 0.36 }}
              size={{ percentage: { width: 0.085, height: 0.18 } }}
              corner={3}
            >
              <div className="relative left-12">
                <AnimatedButterfly fill="#d24d34" duration={8} />
              </div>
              <AnimatedButterfly fill="orange" duration={15} reverse />
            </Frame>
          </div>
        </div>
      </div>

      <div className="relative -top-12 z-10">
        <Outlet />
      </div>
    </div>
  );
}
