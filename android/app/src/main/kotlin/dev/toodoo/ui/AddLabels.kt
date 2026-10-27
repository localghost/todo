package dev.toodoo.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.toodoo.api.Label

/**
 * The order of the chips under the add field: the label of the [filter]
 * first, then the labels with the most items. The same count sorts by name.
 */
fun labelsByUse(labels: List<Label>, filter: Label?): List<Label> =
    labels.sortedWith(
        compareByDescending<Label> { it.id == filter?.id }
            .thenByDescending { it.items ?: 0 }
            .thenBy(String.CASE_INSENSITIVE_ORDER) { it.name },
    )

/**
 * One row of label chips under the add field, scrolled to the side. A tap
 * selects a chip or clears it; the chips stay in their place.
 */
@Composable
fun AddLabelRow(labels: List<Label>, picked: List<Long>, onToggle: (Label) -> Unit) {
    LazyRow(
        contentPadding = PaddingValues(horizontal = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        modifier = Modifier.fillMaxWidth().padding(bottom = 12.dp).semantics { contentDescription = "Labels for the new item" },
    ) {
        items(labels, key = { it.id }) { label -> PickChip(label, label.id in picked) { onToggle(label) } }
    }
}

/** Selected: the label's light fill, a check and a dark border. Not selected: white with a colored dot. */
@Composable
private fun PickChip(label: Label, selected: Boolean, onClick: () -> Unit) {
    val colors = MaterialTheme.colorScheme
    val shades = labelShades(label.color, isSystemInDarkTheme())
    val fill = shades?.let { Color(0xFF000000 or it.fill.toLong()) } ?: colors.surfaceVariant
    val ink = shades?.let { Color(0xFF000000 or it.text.toLong()) } ?: colors.onSurfaceVariant
    FilterChip(
        selected = selected,
        onClick = onClick,
        label = { Text(label.name, fontSize = 15.sp, fontWeight = FontWeight.SemiBold) },
        leadingIcon = {
            if (selected) {
                Icon(Icons.Filled.Check, contentDescription = null, tint = ink, modifier = Modifier.size(18.dp))
            } else {
                Box(Modifier.padding(4.dp).size(10.dp).background(ink, CircleShape))
            }
        },
        shape = RoundedCornerShape(50),
        colors = FilterChipDefaults.filterChipColors(
            containerColor = colors.surface,
            labelColor = colors.onSurface,
            selectedContainerColor = fill,
            selectedLabelColor = ink,
        ),
        border = FilterChipDefaults.filterChipBorder(
            enabled = true,
            selected = selected,
            borderColor = colors.outline,
            selectedBorderColor = ink,
            borderWidth = 1.dp,
            selectedBorderWidth = 2.dp,
        ),
        modifier = Modifier.height(40.dp),
    )
}
