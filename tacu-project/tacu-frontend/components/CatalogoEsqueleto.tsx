import { Card, Skeleton } from "@heroui/react";

/** Los esqueletos mientras la IA lee la carta. */
export function CatalogoEsqueleto() {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 8 }).map((_, i) => (
        <Card key={i} variant="secondary">
          <Skeleton className="h-36 w-full rounded-none" />
          <Card.Content className="gap-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-full" />
            <Skeleton className="h-3 w-1/3" />
          </Card.Content>
        </Card>
      ))}
    </div>
  );
}
