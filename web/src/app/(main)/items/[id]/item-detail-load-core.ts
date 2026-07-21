type ItemDetailLoadOptions<TDetail, TRelated> = {
  loadDetail: () => Promise<TDetail>;
  loadRelated: () => Promise<TRelated>;
  onDetail: (detail: TDetail) => void;
  onDetailError: (error: unknown) => void;
  onRelated: (related: TRelated) => void;
  onRelatedError: (error: unknown) => void;
};

export function startItemDetailLoads<TDetail, TRelated>(
  options: ItemDetailLoadOptions<TDetail, TRelated>,
) {
  const detail = Promise.resolve()
    .then(options.loadDetail)
    .then(options.onDetail, options.onDetailError);
  const related = Promise.resolve()
    .then(options.loadRelated)
    .then(options.onRelated, options.onRelatedError);

  return { detail, related };
}
