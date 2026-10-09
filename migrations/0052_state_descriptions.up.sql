-- Optional description per workflow state, explaining when the state applies
-- (e.g. why work sits in an "On hold" state). Shown as a tooltip wherever
-- states appear: the state picker, board column headers, and settings.
ALTER TABLE project_states
    ADD COLUMN description TEXT;
