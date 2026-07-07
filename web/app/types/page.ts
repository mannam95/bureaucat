// A documentation page as it appears in list responses (content omitted).
export interface PageListItem {
  id: string;
  page_number: number;
  title: string;
  created_by: string;
  creator_username: string;
  creator_first_name: string;
  creator_last_name: string;
  creator_avatar_url?: string;
  created_at: string;
  updated_at: string;
}

// A full documentation page, including its rich (tiptap HTML) content.
export interface Page extends PageListItem {
  project_key: string;
  content: string;
}

export interface CreatePageRequest {
  title: string;
  content?: string;
}

export interface UpdatePageRequest {
  title?: string;
  content?: string;
}
