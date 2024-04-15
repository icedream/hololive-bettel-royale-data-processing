import { MaybeTimeDuration, Duration } from "typed-duration";

export function debounce<A = unknown, R = void>(
    fn: (args: A) => R,
    delay: MaybeTimeDuration
): [(args: A) => Promise<R>, () => void] {
  let timer: NodeJS.Timeout;

  const debouncedFunc = (args: A): Promise<R> =>
    new Promise((resolve) => {
      if (timer) {
          clearTimeout(timer);
      }

      timer = setTimeout(
        () => {
          resolve(fn(args));
        },
        Duration.milliseconds.from(delay),
      );
    });

  const teardown = () => clearTimeout(timer);

  return [debouncedFunc, teardown];
}
