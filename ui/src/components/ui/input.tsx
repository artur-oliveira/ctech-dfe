// standards:allow Thin wrapper over @aoctech/ui Input: the shared Input wires aria from its own Field context, but forms here use shadcn FormItem/FormMessage, whose message id comes from useFieldAria(name).
import * as React from "react";
import {Input as SharedInput} from "@aoctech/ui";

import {useFieldAria} from "@/components/ui/form";

const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<typeof SharedInput>>(
  function Input({name, ...props}, ref) {
    return <SharedInput ref={ref} name={name} {...useFieldAria(name)} {...props} />;
  }
);

export {Input};
