"use client";

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { faq } from "@/lib/content";

export function Faq() {
  return (
    <Accordion className="border-t border-border">
      {faq.map((item, i) => (
        <AccordionItem
          key={item.q}
          value={String(i)}
          style={{ "--i": i } as React.CSSProperties}
          className="reveal-item border-b border-border not-last:border-b"
        >
          <AccordionTrigger className="gap-6 py-4 text-[15px] font-normal transition-colors duration-(--dur-fast) hover:text-primary hover:no-underline">
            {item.q}
          </AccordionTrigger>
          <AccordionContent className="max-w-2xl pb-5 text-[14px] text-pretty text-muted-foreground">
            {item.a}
          </AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  );
}
