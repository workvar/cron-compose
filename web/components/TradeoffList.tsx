import { IconCheck, IconX } from "@/components/icons";

type TradeoffListProps = {
  good: string[];
  bad: string[];
};

// Renders a short pros/cons list under an install command: ticks for what's
// good about that choice, crosses for what it costs you.
export default function TradeoffList({ good, bad }: TradeoffListProps) {
  return (
    <ul className="tradeoff-list">
      {good.map((item) => (
        <li key={item} className="tradeoff-list__item tradeoff-list__item--good">
          <IconCheck className="tradeoff-list__icon" />
          <span>{item}</span>
        </li>
      ))}
      {bad.map((item) => (
        <li key={item} className="tradeoff-list__item tradeoff-list__item--bad">
          <IconX className="tradeoff-list__icon" />
          <span>{item}</span>
        </li>
      ))}
    </ul>
  );
}
