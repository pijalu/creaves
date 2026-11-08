-- #208 item 1: the feeding-guide system ("régime alimentaire" suggestion
-- tables) is removed entirely — handlers, /suggestions/feeding_guide
-- endpoint, admin pages, animal-form modal and menu links are gone.
-- Drop the backing table.
DROP TABLE IF EXISTS `feeding_guides`;
