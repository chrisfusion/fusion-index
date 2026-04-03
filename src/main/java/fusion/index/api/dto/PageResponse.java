package fusion.index.api.dto;

import java.util.List;

public class PageResponse<T> {
    public List<T> items;
    public long total;
    public int page;
    public int pageSize;

    public PageResponse(List<T> items, long total, int page, int pageSize) {
        this.items    = items;
        this.total    = total;
        this.page     = page;
        this.pageSize = pageSize;
    }
}
